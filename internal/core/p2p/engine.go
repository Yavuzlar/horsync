package p2p

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"horsync/internal/config"
	"horsync/internal/core/transfer"
)

// Default P2P ports
const (
	DefaultTCPPort    = 22000
	DefaultUDPPort    = 21027
	handshakeTimeout  = 10 * time.Second
	readDeadline      = 30 * time.Second
	chunkRetryMax     = 3
)

type PeerInfo struct {
	DeviceID string    `json:"deviceId"`
	Name     string    `json:"name"`
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	LastSeen time.Time `json:"lastSeen"`
}

type P2PMessage struct {
	Type        string `json:"type"` // "handshake", "chunk_request", "chunk_response", "heartbeat"
	SenderID    string `json:"senderId"`
	Secret      string `json:"secret,omitempty"`
	ChunkIndex  int    `json:"chunkIndex,omitempty"`
	SessionID   string `json:"sessionId,omitempty"`
	PayloadSize int64  `json:"payloadSize,omitempty"`
}

type PeerConn struct {
	Info   PeerInfo
	Conn   net.Conn
	Writer *json.Encoder
	Reader *json.Decoder
	mu     sync.Mutex
}

type P2PEngine struct {
	mu            sync.RWMutex
	deviceID      string
	deviceName    string
	deviceSecret  string
	port          int
	discoveryPort int
	peers         map[string]*PeerConn
	discovered    map[string]PeerInfo
	ctx           context.Context
	cancel        context.CancelFunc
	tlsConfig     *tls.Config
}

var instance *P2PEngine
var once sync.Once

// GetInstance returns the singleton P2P engine instance.
func GetInstance() *P2PEngine {
	once.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		instance = &P2PEngine{
			port:          DefaultTCPPort,
			discoveryPort: DefaultUDPPort,
			peers:         make(map[string]*PeerConn),
			discovered:    make(map[string]PeerInfo),
			ctx:           ctx,
			cancel:        cancel,
		}
	})
	return instance
}

// Start initializes the P2P engine with device credentials and begins
// discovery and TCP listeners. Pass tcpPort or udpPort == 0 to use the
// engine defaults (22000 / 21027). Non-zero values let multiple agent
// processes coexist on the same host (useful for single-machine tests).
func (p *P2PEngine) Start(deviceID, deviceName, deviceSecret string, tcpPort int, udpPort int) error {
	p.mu.Lock()
	p.deviceID = deviceID
	p.deviceName = deviceName
	p.deviceSecret = deviceSecret
	if tcpPort > 0 {
		p.port = tcpPort
	}
	if udpPort > 0 {
		p.discoveryPort = udpPort
	}
	// Initialize TLS with auto-generated self-signed certificate
	if err := p.initTLS(); err != nil {
		log.Printf("[P2P] WARNING: TLS initialization failed, falling back to plain TCP: %v", err)
		p.tlsConfig = nil
	}
	p.mu.Unlock()

	// Discovery listener and advertiser share the UDP port; on single-host
	// multi-agent setups the second process will fail to bind, which is
	// non-fatal — that process will still dial peers discovered by others.
	go p.runTCPListener()
	go p.runDiscoveryAdvertiser()
	go p.runDiscoveryListener()

	log.Printf("[P2P] Engine initialized for node %s (%s) - TCP:%d UDP:%d", deviceName, deviceID, p.port, p.discoveryPort)
	return nil
}

// IsPeerConnected reports whether a peer with the given device ID is
// currently linked to this P2P engine. Used by the agent replication path
// to decide whether a chunk can be fetched directly from the source peer
// instead of going through the Hub.
func (p *P2PEngine) IsPeerConnected(peerID string) bool {
	if peerID == "" {
		return false
	}
	p.mu.RLock()
	_, ok := p.peers[peerID]
	p.mu.RUnlock()
	return ok
}

// Stop shuts down the P2P engine, cancels all goroutines, and closes all peer connections.
func (p *P2PEngine) Stop() {
	p.cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, pc := range p.peers {
		pc.Conn.Close()
	}
	p.peers = make(map[string]*PeerConn)
}

// GetActivePeers returns a list of currently connected peers.
func (p *P2PEngine) GetActivePeers() []PeerInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	active := make([]PeerInfo, 0, len(p.peers))
	for _, conn := range p.peers {
		active = append(active, conn.Info)
	}
	return active
}

// GetDiscoveredPeers returns peers discovered via UDP multicast within the last minute.
func (p *P2PEngine) GetDiscoveredPeers() []PeerInfo {
	p.mu.RLock()
	defer p.mu.RUnlock()

	discovered := make([]PeerInfo, 0, len(p.discovered))
	for _, info := range p.discovered {
		if time.Since(info.LastSeen) < 1*time.Minute {
			discovered = append(discovered, info)
		}
	}
	return discovered
}

// initTLS generates or loads a self-signed TLS certificate for P2P connections.
func (p *P2PEngine) initTLS() error {
	certFile := filepath.Join("data", "p2p-cert.pem")
	keyFile := filepath.Join("data", "p2p-key.pem")

	// Try loading existing certs first
	if _, err := os.Stat(certFile); err == nil {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err == nil {
			p.tlsConfig = &tls.Config{
				Certificates:       []tls.Certificate{cert},
				InsecureSkipVerify: true, // Self-signed in P2P mesh
				MinVersion:         tls.VersionTLS12,
			}
			log.Printf("[P2P] TLS loaded existing certificate from %s", certFile)
			return nil
		}
		log.Printf("[P2P] Could not load existing cert, generating new one: %v", err)
	}

	// Generate self-signed certificate
	if err := generateSelfSignedCert(certFile, keyFile); err != nil {
		return fmt.Errorf("generate self-signed cert: %w", err)
	}

	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return fmt.Errorf("load generated cert: %w", err)
	}

	p.tlsConfig = &tls.Config{
		Certificates:       []tls.Certificate{cert},
		InsecureSkipVerify: true, // Self-signed in P2P mesh
		MinVersion:         tls.VersionTLS12,
	}
	log.Printf("[P2P] TLS auto-generated self-signed certificate")
	return nil
}

// dialPeer connects to a peer, optionally wrapping with TLS.
func (p *P2PEngine) dialPeer(address string) (net.Conn, error) {
	if p.tlsConfig != nil {
		return tls.DialWithDialer(&net.Dialer{Timeout: 5 * time.Second}, "tcp", address, p.tlsConfig)
	}
	return net.DialTimeout("tcp", address, 5*time.Second)
}

// listenForPeers accepts plain or TLS connections.
func (p *P2PEngine) acceptPeer(listener net.Listener) (net.Conn, error) {
	rawConn, err := listener.Accept()
	if err != nil {
		return nil, err
	}

	if p.tlsConfig != nil {
		tlsConn := tls.Server(rawConn, p.tlsConfig)
		if err := tlsConn.Handshake(); err != nil {
			rawConn.Close()
			return nil, fmt.Errorf("TLS handshake failed: %w", err)
		}
		return tlsConn, nil
	}

	return rawConn, nil
}

// 1. Local Network UDP Multicast Discovery
func (p *P2PEngine) runDiscoveryAdvertiser() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", p.discoveryPort))
	if err != nil {
		log.Printf("[P2P] Failed to resolve broadcast address: %v", err)
		return
	}

	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Printf("[P2P] Failed to dial UDP broadcast: %v", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-p.ctx.Done():
			return
		case <-ticker.C:
			p.mu.RLock()
			info := PeerInfo{
				DeviceID: p.deviceID,
				Name:     p.deviceName,
				Port:     p.port,
			}
			p.mu.RUnlock()

			payload, err := json.Marshal(info)
			if err == nil {
				_, _ = conn.Write(payload)
			}
		}
	}
}

func (p *P2PEngine) runDiscoveryListener() {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("0.0.0.0:%d", p.discoveryPort))
	if err != nil {
		log.Printf("[P2P] Failed to resolve UDP bind address: %v", err)
		return
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		log.Printf("[P2P] Failed to listen UDP discovery: %v", err)
		return
	}
	defer conn.Close()

	buffer := make([]byte, 1024)
	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			n, remoteAddr, err := conn.ReadFromUDP(buffer)
			if err != nil {
				continue
			}

			var info PeerInfo
			if err := json.Unmarshal(buffer[:n], &info); err == nil {
				p.mu.RLock()
				myID := p.deviceID
				p.mu.RUnlock()

				if info.DeviceID != myID {
					info.IP = remoteAddr.IP.String()
					info.LastSeen = time.Now()

					p.mu.Lock()
					p.discovered[info.DeviceID] = info
					p.mu.Unlock()

					// Try connecting to discovered peer
					go p.TryConnect(info)
				}
			}
		}
	}
}

// 2. TCP Direct (optionally TLS) Connection broker
func (p *P2PEngine) runTCPListener() {
	listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", p.port))
	if err != nil {
		log.Printf("[P2P] Failed to start P2P TCP Listener: %v", err)
		return
	}
	defer listener.Close()

	log.Printf("[P2P] P2P listener active on port %d (TLS: %v)", p.port, p.tlsConfig != nil)

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
			conn, err := p.acceptPeer(listener)
			if err != nil {
				continue
			}
			go p.handleIncomingConnection(conn)
		}
	}
}

// TryConnect attempts to establish a connection to a discovered peer.
func (p *P2PEngine) TryConnect(target PeerInfo) {
	p.mu.Lock()
	if _, connected := p.peers[target.DeviceID]; connected {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	conn, err := p.dialPeer(fmt.Sprintf("%s:%d", target.IP, target.Port))
	if err != nil {
		return
	}

	p.negotiateHandshake(conn, target, true)
}

func (p *P2PEngine) handleIncomingConnection(conn net.Conn) {
	p.negotiateHandshake(conn, PeerInfo{}, false)
}

func (p *P2PEngine) negotiateHandshake(conn net.Conn, info PeerInfo, isOutgoing bool) {
	reader := json.NewDecoder(conn)
	writer := json.NewEncoder(conn)

	p.mu.RLock()
	myID := p.deviceID
	mySecret := p.deviceSecret
	p.mu.RUnlock()

	// 1. Send Handshake
	handshake := P2PMessage{
		Type:     "handshake",
		SenderID: myID,
		Secret:   mySecret,
	}

	_ = conn.SetDeadline(time.Now().Add(handshakeTimeout))
	if err := writer.Encode(handshake); err != nil {
		conn.Close()
		return
	}

	// 2. Receive Handshake
	var response P2PMessage
	if err := reader.Decode(&response); err != nil || response.Type != "handshake" {
		conn.Close()
		return
	}

	// 3. Authenticate Link (Strict Approval Mode)
	db := config.GetDatabase()
	if db != nil {
		if settings, err := db.GetInstanceSettings(context.Background()); err == nil && settings.P2PStrictApproval {
			var status string
			err := db.Pool.QueryRow(context.Background(), "SELECT status FROM devices WHERE device_id = $1", response.SenderID).Scan(&status)
			if err != nil || status != "active" {
				log.Printf("[P2P] Security Rejection: Connection from pending/unapproved node %s terminated.", response.SenderID)
				conn.Close()
				return
			}
		}
	}

	_ = conn.SetDeadline(time.Time{}) // clear deadline

	p.mu.Lock()
	if info.DeviceID == "" {
		info.DeviceID = response.SenderID
		if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			info.IP = tcpAddr.IP.String()
		}
		info.Port = p.port
	}
	info.LastSeen = time.Now()

	pc := &PeerConn{
		Info:   info,
		Conn:   conn,
		Writer: writer,
		Reader: reader,
	}
	p.peers[info.DeviceID] = pc
	p.mu.Unlock()

	log.Printf("[P2P] Established secure link to peer node: %s (%s)", info.Name, info.DeviceID)
	go p.listenToPeer(pc)
}

func (p *P2PEngine) listenToPeer(pc *PeerConn) {
	defer func() {
		pc.Conn.Close()
		p.mu.Lock()
		delete(p.peers, pc.Info.DeviceID)
		p.mu.Unlock()
		log.Printf("[P2P] Disconnected peer node: %s", pc.Info.Name)
	}()

	for {
		var msg P2PMessage
		if err := pc.Reader.Decode(&msg); err != nil {
			return
		}

		switch msg.Type {
		case "chunk_request":
			go p.handleChunkRequest(pc, msg)
		case "heartbeat":
			pc.mu.Lock()
			pc.Info.LastSeen = time.Now()
			pc.mu.Unlock()
		}
	}
}

// 3. Block Exchange Protocol - Real Chunk Reader
// Reads the actual file chunk from disk for P2P transfer.
// The on-disk path is resolved from the database via the transfer service so
// that it matches the path used at upload time (data/uploads/{sessionId}/{fileName}.part),
// rather than the legacy wrong "{sessionId}.part" layout.
func (p *P2PEngine) handleChunkRequest(pc *PeerConn, msg P2PMessage) {
	chunkPath, pathErr := transfer.GetInstance().ResolveUploadFile(context.Background(), msg.SessionID)
	if pathErr != nil {
		log.Printf("[P2P] Failed to resolve upload path for session %s: %v", msg.SessionID, pathErr)
		chunkPath = ""
	}

	log.Printf("[P2P] Serving chunk request: session=%s index=%d path=%s", msg.SessionID, msg.ChunkIndex, chunkPath)

	var payload []byte

	// Try to read the chunk from the actual file on disk
	if chunkPath == "" {
		payload = make([]byte, 0)
	} else {
		file, err := os.Open(chunkPath)
		if err == nil {
			defer file.Close()

			chunkSize := int(msg.PayloadSize)
			if chunkSize <= 0 {
				chunkSize = 2 * 1024 * 1024 // Default 2MB
			}
			payload = make([]byte, chunkSize)
			offset := int64(msg.ChunkIndex) * int64(chunkSize)
			n, err := file.ReadAt(payload, offset)
			if err != nil && err != io.EOF {
				log.Printf("[P2P] Error reading chunk %d: %v", msg.ChunkIndex, err)
				payload = nil
			} else {
				payload = payload[:n]
			}
		} else {
			log.Printf("[P2P] Chunk file not found at %s, returning empty payload: %v", chunkPath, err)
			// Return empty payload for non-existent files to allow connection verification
			payload = make([]byte, 0)
		}
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	response := P2PMessage{
		Type:        "chunk_response",
		SenderID:    p.deviceID,
		SessionID:   msg.SessionID,
		ChunkIndex:  msg.ChunkIndex,
		PayloadSize: int64(len(payload)),
	}

	_ = pc.Conn.SetDeadline(time.Now().Add(readDeadline))
	defer func() { _ = pc.Conn.SetDeadline(time.Time{}) }()

	if err := pc.Writer.Encode(response); err == nil && len(payload) > 0 {
		_, _ = pc.Conn.Write(payload)
	}
}

// PullChunkFromPeer requests a file chunk from a connected peer and returns the raw bytes.
func (p *P2PEngine) PullChunkFromPeer(peerID string, sessionID string, chunkIndex int, chunkSize int) ([]byte, error) {
	p.mu.RLock()
	pc, connected := p.peers[peerID]
	p.mu.RUnlock()

	if !connected {
		return nil, fmt.Errorf("peer %s not connected", peerID)
	}

	pc.mu.Lock()
	defer pc.mu.Unlock()

	request := P2PMessage{
		Type:        "chunk_request",
		SenderID:    p.deviceID,
		SessionID:   sessionID,
		ChunkIndex:  chunkIndex,
		PayloadSize: int64(chunkSize),
	}

	_ = pc.Conn.SetDeadline(time.Now().Add(readDeadline))
	defer func() { _ = pc.Conn.SetDeadline(time.Time{}) }()

	var lastErr error
	for attempt := 0; attempt < chunkRetryMax; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
		}

		if err := pc.Writer.Encode(request); err != nil {
			lastErr = err
			continue
		}

		var response P2PMessage
		if err := pc.Reader.Decode(&response); err != nil || response.Type != "chunk_response" {
			lastErr = fmt.Errorf("failed to read chunk response: %w", err)
			continue
		}

		if response.PayloadSize <= 0 {
			return []byte{}, nil // Empty chunk
		}

		payload := make([]byte, response.PayloadSize)
		_, err := io.ReadFull(pc.Conn, payload)
		if err != nil {
			lastErr = fmt.Errorf("failed to read chunk stream bytes: %w", err)
			continue
		}

		return payload, nil
	}

	return nil, fmt.Errorf("chunk pull failed after %d retries: %w", chunkRetryMax, lastErr)
}
