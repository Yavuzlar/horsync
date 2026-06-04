package p2p

import (
	"encoding/json"
	"net"
	"testing"
	"time"
)

func TestP2PMessageSerialization(t *testing.T) {
	msg := P2PMessage{
		Type:        "handshake",
		SenderID:    "test-sender-node",
		Secret:      "test-secret",
		ChunkIndex:  4,
		SessionID:   "session-abc",
		PayloadSize: 2048,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Failed to marshal P2PMessage: %v", err)
	}

	var parsed P2PMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal P2PMessage: %v", err)
	}

	if parsed.Type != "handshake" || parsed.SenderID != "test-sender-node" || parsed.Secret != "test-secret" || parsed.ChunkIndex != 4 || parsed.SessionID != "session-abc" || parsed.PayloadSize != 2048 {
		t.Errorf("Deserialized P2PMessage fields do not match original: %+v", parsed)
	}
}

func TestP2PHandshakeNegotiation(t *testing.T) {
	// Create mock tcp network connection to verify handshake protocol exchange
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Failed to start listener: %v", err)
	}
	defer listener.Close()
	listenAddr := listener.Addr().String()

	errChan := make(chan error, 2)

	// Server-side negotiation
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			errChan <- err
			return
		}
		defer conn.Close()

		reader := json.NewDecoder(conn)
		writer := json.NewEncoder(conn)

		// Read client's handshake
		var clientHandshake P2PMessage
		if err := reader.Decode(&clientHandshake); err != nil {
			errChan <- err
			return
		}

		if clientHandshake.Type != "handshake" || clientHandshake.SenderID != "client-node-1" {
			t.Errorf("Unexpected client handshake message: %+v", clientHandshake)
		}

		// Write server's handshake
		serverHandshake := P2PMessage{
			Type:     "handshake",
			SenderID: "server-node-1",
		}
		if err := writer.Encode(serverHandshake); err != nil {
			errChan <- err
			return
		}

		errChan <- nil
	}()

	// Client-side negotiation
	go func() {
		conn, err := net.Dial("tcp", listenAddr)
		if err != nil {
			errChan <- err
			return
		}
		defer conn.Close()

		reader := json.NewDecoder(conn)
		writer := json.NewEncoder(conn)

		// Write client's handshake
		clientHandshake := P2PMessage{
			Type:     "handshake",
			SenderID: "client-node-1",
			Secret:   "client-secret",
		}
		if err := writer.Encode(clientHandshake); err != nil {
			errChan <- err
			return
		}

		// Read server's handshake
		var serverHandshake P2PMessage
		if err := reader.Decode(&serverHandshake); err != nil {
			errChan <- err
			return
		}

		if serverHandshake.Type != "handshake" || serverHandshake.SenderID != "server-node-1" {
			t.Errorf("Unexpected server handshake response: %+v", serverHandshake)
		}

		errChan <- nil
	}()

	// Wait for both routines to complete
	for i := 0; i < 2; i++ {
		select {
		case err := <-errChan:
			if err != nil {
				t.Fatalf("Handshake simulation failed: %v", err)
			}
		case <-time.After(3 * time.Second):
			t.Fatal("Handshake negotiation timed out")
		}
	}
}
