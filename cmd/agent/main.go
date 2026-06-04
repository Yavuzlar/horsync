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
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"horsync/internal/models"
)

type agentConfig struct {
	baseURL      string
	deviceID     string
	deviceSecret string
	storageDir   string
	pollInterval time.Duration
}

func main() {
	cfg := loadConfig()
	if cfg.deviceID == "" || cfg.deviceSecret == "" {
		log.Fatal("device credentials are required")
	}

	if err := os.MkdirAll(cfg.storageDir, 0o755); err != nil {
		log.Fatalf("prepare storage dir: %v", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	log.Printf("node agent started for %s", cfg.deviceID)

	for {
		if err := pollOnce(context.Background(), client, cfg); err != nil {
			log.Printf("poll error: %v", err)
		}
		time.Sleep(cfg.pollInterval)
	}
}

func loadConfig() agentConfig {
	var cfg agentConfig
	flag.StringVar(&cfg.baseURL, "base-url", envOrDefault("AGENT_BASE_URL", "http://localhost:3001"), "control plane base URL")
	flag.StringVar(&cfg.deviceID, "device-id", os.Getenv("AGENT_DEVICE_ID"), "registered device id")
	flag.StringVar(&cfg.deviceSecret, "device-secret", os.Getenv("AGENT_DEVICE_SECRET"), "registered device secret")
	flag.StringVar(&cfg.storageDir, "storage-dir", envOrDefault("AGENT_STORAGE_DIR", "data/replicated"), "replicated file storage directory")
	pollSeconds := flag.Int("poll-seconds", envIntOrDefault("AGENT_POLL_SECONDS", 20), "poll interval in seconds")
	flag.Parse()
	cfg.pollInterval = time.Duration(*pollSeconds) * time.Second
	if cfg.pollInterval < 5*time.Second {
		cfg.pollInterval = 5 * time.Second
	}
	return cfg
}

func pollOnce(ctx context.Context, client *http.Client, cfg agentConfig) error {
	var jobs []models.ReplicationJob
	if err := requestJSON(ctx, client, cfg, http.MethodGet, "/api/agent/jobs", nil, &jobs); err != nil {
		return err
	}

	if len(jobs) == 0 {
		log.Printf("no replication jobs")
		return nil
	}

	for _, job := range jobs {
		if err := processJob(ctx, client, cfg, job); err != nil {
			log.Printf("job %s failed: %v", job.ID, err)
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
		payload, headerHash, err := downloadChunk(ctx, client, cfg, job.ID, chunk.ChunkIndex)
		if err != nil {
			return ackFailure(ctx, client, cfg, job.ID, err)
		}

		actualHash := hashBytes(payload)
		if headerHash != "" && headerHash != actualHash {
			return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("chunk %d header sha mismatch", chunk.ChunkIndex))
		}
		if chunk.ChunkSHA256 != "" && chunk.ChunkSHA256 != actualHash {
			return ackFailure(ctx, client, cfg, job.ID, fmt.Errorf("chunk %d manifest sha mismatch", chunk.ChunkIndex))
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

func downloadChunk(ctx context.Context, client *http.Client, cfg agentConfig, jobID string, chunkIndex int) ([]byte, string, error) {
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

func envOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}

	var parsed int
	if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

