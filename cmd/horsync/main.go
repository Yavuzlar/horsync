package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"horsync/internal/api/routes"
	"horsync/internal/config"
	"horsync/internal/core/engine"
	"horsync/internal/core/p2p"
	"horsync/internal/core/sysmonitor"
	"horsync/internal/core/topology"
	"horsync/internal/core/transfer"
	"horsync/internal/core/vault"
	"horsync/internal/models"
	"horsync/pkg/logger"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
)

type agentConfig struct {
	baseURL      string
	deviceID     string
	deviceSecret string
	storageDir   string
	pollInterval time.Duration
}

func main() {
	// Parse CLI options
	isAgent := flag.Bool("agent", false, "Run in background sync Agent mode")
	install := flag.Bool("install", false, "Automatically detect host OS and configure as background autostart service")
	uninstall := flag.Bool("uninstall", false, "Remove background autostart service registration")

	deviceID := flag.String("device-id", "", "Agent device identifier")
	deviceSecret := flag.String("device-secret", "", "Agent secret key")
	baseURL := flag.String("base-url", "http://localhost:3001", "Central control plane URL")
	storageDir := flag.String("storage-dir", "data/replicated", "Replicated storage directory")
	pollSeconds := flag.Int("poll-seconds", 10, "Agent central polling interval in seconds")
	p2pTCPPort := flag.Int("p2p-tcp-port", p2p.DefaultTCPPort, "P2P mesh TCP listen port (use a unique port per agent on the same host)")
	p2pUDPPort := flag.Int("p2p-udp-port", p2p.DefaultUDPPort, "P2P mesh UDP discovery port (use a unique port per agent on the same host)")

	flag.Parse()

	// 1. Process Service Installation
	if *install {
		installService(*deviceID, *deviceSecret, *baseURL, *storageDir, *pollSeconds, *p2pTCPPort, *p2pUDPPort)
		return
	}

	// 2. Process Service Uninstallation
	if *uninstall {
		uninstallService()
		return
	}

	// 3. Run corresponding mode
	if *isAgent {
		runAgent(*deviceID, *deviceSecret, *baseURL, *storageDir, *pollSeconds, *p2pTCPPort, *p2pUDPPort)
	} else {
		// Default mode: run as central Hub server
		runServer()
	}
}

// ==========================================
// Central Hub Server Mode Execution
// ==========================================
func runServer() {
	appConfig := config.Load()

	logger.Init(logger.Config{
		Level:      slog.LevelDebug,
		IsJSON:     false,
		LogToFile:  true,
		FilePath:   "logs/server.log",
		MaxSize:    100,
		MaxBackups: 5,
		Service:    "HORSYNC-UNIFIED",
	})

	logger.L.Info("Unified server runtime active. Core services starting...")

	if appConfig.DatabaseURL != "" {
		db, err := config.InitDatabase(context.Background(), appConfig.DatabaseURL)
		if err != nil {
			logger.L.Error("PostgreSQL connection failed.", "error", err)
		} else {
			if err := db.Migrate(context.Background()); err != nil {
				logger.L.Error("PostgreSQL migration failed.", "error", err)
			} else {
				logger.L.Info("PostgreSQL control plane ready.", "database", "connected")
			}
			defer db.Close()
		}
	} else {
		logger.L.Warn("DATABASE_URL not defined. Control plane endpoints will be disabled.")
	}

	sysmonitor.GetInstance().Start()
	engine.GetInstance().Start()
	topology.GetInstance().Start()
	vault.GetInstance().Start()
	_ = p2p.GetInstance().Start("YVS-HUB-CORE-PLANE", "Horsync Central Hub", "central-hub-secret", p2p.DefaultTCPPort, p2p.DefaultUDPPort)
	if err := transfer.GetInstance().Start(); err != nil {
		logger.L.Error("Upload storage failed to initialize.", "error", err)
	}

	bodyLimit := appConfig.MaxChunkSizeBytes + (256 * 1024)
	if bodyLimit < 1024*1024 {
		bodyLimit = 1024 * 1024
	}
	app := fiber.New(fiber.Config{
		AppName:   "Horsync Core Service",
		BodyLimit: bodyLimit,
	})

	app.Use(cors.New())

	routes.Register(app)

	distDir := filepath.Join("frontend", "dist")
	if _, err := os.Stat(filepath.Join(distDir, "index.html")); err == nil {
		app.Use("/", filesystem.New(filesystem.Config{
			Root:   http.Dir(distDir),
			Browse: false,
		}))

		app.Get("*", func(c *fiber.Ctx) error {
			if strings.HasPrefix(c.Path(), "/api") {
				return c.SendStatus(fiber.StatusNotFound)
			}

			return c.SendFile(filepath.Join(distDir, "index.html"))
		})
		logger.L.Info("Frontend static distribution linked successfully.", "path", distDir)
	} else {
		logger.L.Warn("Frontend distribution not found. UI needs dynamic front-end build.", "path", distDir)
	}

	logger.L.Info("HTTP control plane active.", "addr", appConfig.AppAddr)
	if err := app.Listen(appConfig.AppAddr); err != nil {
		logger.L.Error("HTTP control plane failed to start.", "error", err)
	}
}

// ==========================================
// P2P Replication Agent Mode Execution
// ==========================================
func runAgent(deviceID, deviceSecret, baseURL, storageDir string, pollSeconds, tcpPort, udpPort int) {
	if deviceID == "" || deviceSecret == "" {
		log.Fatal("[HORSYNC-AGENT] Device credentials (--device-id, --device-secret) are required")
	}

	if err := os.MkdirAll(storageDir, 0o755); err != nil {
		log.Fatalf("[HORSYNC-AGENT] Prepare storage dir: %v", err)
	}

	// Bring up the P2P mesh engine so this agent advertises itself on the
	// LAN and can both serve and pull chunks directly from other peers.
	// Each agent on the same host MUST use unique TCP/UDP ports, otherwise
	// the second process fails to bind (logged as non-fatal).
	if err := p2p.GetInstance().Start(deviceID, deviceID, deviceSecret, tcpPort, udpPort); err != nil {
		log.Printf("[HORSYNC-AGENT] P2P engine failed to start (continue with Hub-only replication): %v", err)
	}

	cfg := agentConfig{
		baseURL:      baseURL,
		deviceID:     deviceID,
		deviceSecret: deviceSecret,
		storageDir:   storageDir,
		pollInterval: time.Duration(pollSeconds) * time.Second,
	}

	client := &http.Client{Timeout: 30 * time.Second}
	log.Printf("[HORSYNC-AGENT] Node Agent started successfully for Device: %s (P2P TCP:%d UDP:%d)", cfg.deviceID, tcpPort, udpPort)

	for {
		if err := pollOnce(context.Background(), client, cfg); err != nil {
			log.Printf("[HORSYNC-AGENT] Sync Poll Error: %v", err)
		}
		time.Sleep(cfg.pollInterval)
	}
}

func pollOnce(ctx context.Context, client *http.Client, cfg agentConfig) error {
	var jobs []models.ReplicationJob
	if err := requestJSON(ctx, client, cfg, http.MethodGet, "/api/agent/jobs", nil, &jobs); err != nil {
		return err
	}

	if len(jobs) == 0 {
		return nil
	}

	for _, job := range jobs {
		log.Printf("[HORSYNC-AGENT] Processing active replication job: %s", job.ID)
		if err := processJob(ctx, client, cfg, job); err != nil {
			log.Printf("[HORSYNC-AGENT] Replication Job %s Failed: %v", job.ID, err)
		} else {
			log.Printf("[HORSYNC-AGENT] Replication Job %s successfully processed and synced!", job.ID)
		}
	}

	return nil
}

func processJob(ctx context.Context, client *http.Client, cfg agentConfig, job models.ReplicationJob) error {
	var manifest models.ReplicationManifest
	if err := requestJSON(ctx, client, cfg, http.MethodGet, "/api/agent/jobs/"+job.ID+"/manifest", nil, &manifest); err != nil {
		return err
	}

	filePath := filepath.Join(cfg.storageDir, cfg.deviceID, manifest.SessionID, filepath.Base(manifest.FileName))
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("prepare replication dir: %w", err))
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("open replication file: %w", err))
	}
	defer file.Close()

	if err := file.Truncate(manifest.TotalSize); err != nil {
		return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("allocate replication file: %w", err))
	}

	for _, chunk := range manifest.Chunks {
		payload, source, err := downloadChunkP2PThenHub(ctx, client, cfg, manifest, chunk.ChunkIndex)
		if err != nil {
			return ackFailure(ctx, client, cfg, job.ID, err)
		}

		actualHash := hashBytes(payload)
		// P2P responses do not carry HTTP headers, so we only enforce the
		// manifest hash for the direct chunk path. Hub responses additionally
		// cross-check the X-Chunk-SHA256 header (already hashed by caller).
		if chunk.ChunkSHA256 != "" && chunk.ChunkSHA256 != actualHash {
			return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("chunk %d manifest sha mismatch (source=%s)", chunk.ChunkIndex, source))
		}

		if source == "p2p" {
			log.Printf("[HORSYNC-AGENT] Chunk %d fetched via P2P from peer %s", chunk.ChunkIndex, manifest.SourceDeviceID)
		}

		offset := int64(chunk.ChunkIndex) * int64(manifest.ChunkSize)
		if _, err := file.WriteAt(payload, offset); err != nil {
			return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("write chunk %d: %w", chunk.ChunkIndex, err))
		}
	}

	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("seek replication file: %w", err))
	}

	hasher := sha256.New()
	if _, err := io.CopyN(hasher, file, manifest.TotalSize); err != nil {
		return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("hash replication file: %w", err))
	}
	finalHash := hex.EncodeToString(hasher.Sum(nil))
	if manifest.ExpectedSHA256 != "" && !strings.EqualFold(manifest.ExpectedSHA256, finalHash) {
		return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("final sha256 mismatch"))
	}

	return ackSuccess(ctx, client, cfg, job.ID, finalHash)
}

// downloadChunkP2PThenHub fetches a single replication chunk, preferring a
// direct peer-to-peer fetch from the source device when that peer is
// currently linked in the P2P mesh. Falls back to the central Hub HTTP
// endpoint when the source peer is offline or the P2P exchange fails.
// Returns ("p2p" or "hub", error) so the caller can audit the source.
func downloadChunkP2PThenHub(ctx context.Context, client *http.Client, cfg agentConfig, manifest models.ReplicationManifest, chunkIndex int) ([]byte, string, error) {
	// 1. Try a direct P2P pull from the source device, if available.
	if manifest.SourceDeviceID != "" && p2p.GetInstance().IsPeerConnected(manifest.SourceDeviceID) {
		payload, err := p2p.GetInstance().PullChunkFromPeer(manifest.SourceDeviceID, manifest.SessionID, chunkIndex, manifest.ChunkSize)
		if err != nil {
			log.Printf("[HORSYNC-AGENT] P2P pull for chunk %d failed, falling back to Hub: %v", chunkIndex, err)
		} else {
			return payload, "p2p", nil
		}
	}

	// 2. Fall back to the central Hub.
	payload, headerHash, err := downloadChunkFromHub(ctx, client, cfg, manifest.JobID, chunkIndex)
	if err != nil {
		return nil, "", err
	}

	// Cross-check the Hub-provided header hash if present.
	if headerHash != "" {
		actualHash := hashBytes(payload)
		if headerHash != actualHash {
			return nil, "", fmt.Errorf("chunk %d header sha mismatch", chunkIndex)
		}
	}

	return payload, "hub", nil
}

func downloadChunkFromHub(ctx context.Context, client *http.Client, cfg agentConfig, jobID string, chunkIndex int) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(cfg.baseURL, "/")+fmt.Sprintf("/api/agent/jobs/%s/chunks/%d", jobID, chunkIndex), nil)
	if err != nil {
		return nil, "", err
	}
	applyDeviceHeaders(req, cfg)

	res, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return nil, "", fmt.Errorf("download chunk %d failed: %s", chunkIndex, strings.TrimSpace(string(body)))
	}

	payload, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, "", err
	}

	return payload, strings.TrimSpace(res.Header.Get("X-Chunk-SHA256")), nil
}

func ackSuccess(ctx context.Context, client *http.Client, cfg agentConfig, jobID string, finalHash string) error {
	input := models.ReplicationAckInput{
		Status:         "committed",
		VerifiedSHA256: finalHash,
	}
	return postAck(ctx, client, cfg, jobID, input)
}

func ackFailure(ctx context.Context, client *http.Client, cfg agentConfig, jobID string, cause error) error {
	input := models.ReplicationAckInput{
		Status:    "failed",
		LastError: cause.Error(),
	}
	if ackErr := postAck(ctx, client, cfg, jobID, input); ackErr != nil {
		return fmt.Errorf("%v; ack failed: %w", cause, ackErr)
	}
	return cause
}

func postAck(ctx context.Context, client *http.Client, cfg agentConfig, jobID string, input models.ReplicationAckInput) error {
	payload, err := json.Marshal(input)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(cfg.baseURL, "/")+"/api/agent/jobs/"+jobID+"/complete", bytes.NewReader(payload))
	if err != nil {
		return err
	}
	applyDeviceHeaders(req, cfg)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(res.Body, 2048))
		return fmt.Errorf("ack failed: %s", strings.TrimSpace(string(body)))
	}

	return nil
}

func requestJSON(ctx context.Context, client *http.Client, cfg agentConfig, method string, path string, body io.Reader, target any) error {
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(cfg.baseURL, "/")+path, body)
	if err != nil {
		return err
	}
	applyDeviceHeaders(req, cfg)

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("request failed: %s", strings.TrimSpace(string(payload)))
	}

	return json.NewDecoder(res.Body).Decode(target)
}

func applyDeviceHeaders(req *http.Request, cfg agentConfig) {
	req.Header.Set("X-Device-ID", cfg.deviceID)
	req.Header.Set("X-Device-Secret", cfg.deviceSecret)
}

func hashBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// ==========================================
// Multi-OS Persistence Service Installer
// ==========================================
func installService(deviceID, deviceSecret, baseURL, storageDir string, pollSeconds, tcpPort, udpPort int) {
	if deviceID == "" || deviceSecret == "" {
		log.Fatal("[INSTALL] Device credentials (--device-id, --device-secret) are required for installation")
	}

	execPath, err := os.Executable()
	if err != nil {
		log.Fatalf("[INSTALL] Failed to retrieve current executable path: %v", err)
	}
	absExecPath, err := filepath.Abs(execPath)
	if err != nil {
		absExecPath = execPath
	}

	hostOS := runtime.GOOS
	fmt.Printf("[INSTALL] Host operating system detected: %s\n", strings.ToUpper(hostOS))
	fmt.Printf("[INSTALL] Executable target: %s\n", absExecPath)

	switch hostOS {
	case "windows":
		// Windows Registry Autorun Setup
		// Escapes parameters safely to run completely silently in background
		value := fmt.Sprintf(`"%s" --agent --device-id="%s" --device-secret="%s" --base-url="%s" --storage-dir="%s" --poll-seconds=%d --p2p-tcp-port=%d --p2p-udp-port=%d`,
			absExecPath, deviceID, deviceSecret, baseURL, storageDir, pollSeconds, tcpPort, udpPort)

		cmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "HorsyncAgent", "/t", "REG_SZ", "/d", value, "/f")
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Fatalf("[INSTALL] Windows Registry configuration failed: %v, Output: %s", err, string(output))
		}
		fmt.Println("[INSTALL] SUCCESS: Windows User Autostart Registry key added successfully!")
		fmt.Println("[INSTALL] The Horsync Sync Agent will now launch silently in the background on user login.")

	case "linux":
		// Linux Systemd User Daemon Setup
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("[INSTALL] Failed to get user home directory: %v", err)
		}

		systemdDir := filepath.Join(home, ".config", "systemd", "user")
		if err := os.MkdirAll(systemdDir, 0o755); err != nil {
			log.Fatalf("[INSTALL] Failed to create systemd user config directory: %v", err)
		}

		serviceContent := fmt.Sprintf(`[Unit]
Description=Horsync Background Sync Agent
After=network.target

[Service]
ExecStart=%s --agent --device-id=%s --device-secret=%s --base-url=%s --storage-dir=%s --poll-seconds=%d --p2p-tcp-port=%d --p2p-udp-port=%d
Restart=always
RestartSec=10

[Install]
WantedBy=default.target
`, absExecPath, deviceID, deviceSecret, baseURL, storageDir, pollSeconds, tcpPort, udpPort)

		serviceFile := filepath.Join(systemdDir, "horsync.service")
		if err := os.WriteFile(serviceFile, []byte(serviceContent), 0o644); err != nil {
			log.Fatalf("[INSTALL] Failed to write systemd service file: %v", err)
		}

		// Reload and enable the systemd service
		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		_ = exec.Command("systemctl", "--user", "enable", "horsync").Run()
		_ = exec.Command("systemctl", "--user", "start", "horsync").Run()

		fmt.Println("[INSTALL] SUCCESS: Linux Systemd User Service deployed and enabled!")
		fmt.Println("[INSTALL] Verify daemon running state via: systemctl --user status horsync")

	case "darwin":
		// macOS LaunchAgent Daemon Setup
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("[INSTALL] Failed to get user home directory: %v", err)
		}

		launchAgentDir := filepath.Join(home, "Library", "LaunchAgents")
		if err := os.MkdirAll(launchAgentDir, 0o755); err != nil {
			log.Fatalf("[INSTALL] Failed to create macOS LaunchAgents directory: %v", err)
		}

		plistContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>local.horsync</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>--agent</string>
		<string>--device-id=%s</string>
		<string>--device-secret=%s</string>
		<string>--base-url=%s</string>
		<string>--storage-dir=%s</string>
		<string>--poll-seconds=%d</string>
		<string>--p2p-tcp-port=%d</string>
		<string>--p2p-udp-port=%d</string>
	</array>
	<key>KeepAlive</key>
	<true/>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, absExecPath, deviceID, deviceSecret, baseURL, storageDir, pollSeconds, tcpPort, udpPort)

		plistFile := filepath.Join(launchAgentDir, "local.horsync.plist")
		if err := os.WriteFile(plistFile, []byte(plistContent), 0o644); err != nil {
			log.Fatalf("[INSTALL] Failed to write macOS plist file: %v", err)
		}

		_ = exec.Command("launchctl", "load", "-w", plistFile).Run()

		fmt.Println("[INSTALL] SUCCESS: macOS LaunchAgent daemon loaded successfully!")
		fmt.Println("[INSTALL] The Horsync Agent will start automatically in background on user login.")

	default:
		log.Fatalf("[INSTALL] Operating system %s not natively supported for automated service installation.", hostOS)
	}
}

func uninstallService() {
	hostOS := runtime.GOOS
	fmt.Printf("[UNINSTALL] Host operating system detected: %s\n", strings.ToUpper(hostOS))

	switch hostOS {
	case "windows":
		cmd := exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "HorsyncAgent", "/f")
		if output, err := cmd.CombinedOutput(); err != nil {
			log.Fatalf("[UNINSTALL] Windows Registry cleanup failed (it may not have been installed): %v, Output: %s", err, string(output))
		}
		fmt.Println("[UNINSTALL] SUCCESS: Windows User Autostart Registry entry successfully removed!")

	case "linux":
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("[UNINSTALL] Failed to get user home directory: %v", err)
		}

		_ = exec.Command("systemctl", "--user", "stop", "horsync").Run()
		_ = exec.Command("systemctl", "--user", "disable", "horsync").Run()

		serviceFile := filepath.Join(home, ".config", "systemd", "user", "horsync.service")
		_ = os.Remove(serviceFile)

		_ = exec.Command("systemctl", "--user", "daemon-reload").Run()
		fmt.Println("[UNINSTALL] SUCCESS: Linux Systemd user service successfully disabled and deleted!")

	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			log.Fatalf("[UNINSTALL] Failed to get user home directory: %v", err)
		}

		plistFile := filepath.Join(home, "Library", "LaunchAgents", "local.horsync.plist")
		_ = exec.Command("launchctl", "unload", "-w", plistFile).Run()
		_ = os.Remove(plistFile)

		fmt.Println("[UNINSTALL] SUCCESS: macOS LaunchAgent plist successfully unloaded and deleted!")

	default:
		log.Fatalf("[UNINSTALL] Operating system %s not supported for automated cleanup.", hostOS)
	}
}

