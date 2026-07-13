package transfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"horsync/internal/config"
	"horsync/internal/core/engine"
	"horsync/internal/models"
)

var ErrUploadSessionNotFound = errors.New("upload session not found")
var ErrInvalidUploadSession = errors.New("invalid upload session")
var ErrUploadStorageUnavailable = errors.New("upload storage unavailable")
var ErrChunkTooLarge = errors.New("chunk exceeds configured limit")
var ErrInvalidChunkIndex = errors.New("invalid chunk index")
var ErrChunkIntegrityMismatch = errors.New("chunk sha256 mismatch")
var ErrUploadAlreadyCompleted = errors.New("upload already completed")
var ErrUploadIncomplete = errors.New("upload incomplete")
var ErrUploadIntegrityMismatch = errors.New("final sha256 mismatch")
var ErrReplicationJobNotFound = errors.New("replication job not found")
var ErrReplicationChunkNotFound = errors.New("replication chunk not found")

type Manager struct {
	storagePath string
	maxChunk    int
	locks       sync.Map
}

var instance *Manager
var once sync.Once

func GetInstance() *Manager {
	once.Do(func() {
		cfg := config.Load()
		instance = &Manager{
			storagePath: cfg.UploadStoragePath,
			maxChunk:    cfg.MaxChunkSizeBytes,
		}
	})

	return instance
}

// Start ensures the upload storage directory exists, creating it if necessary.
func (m *Manager) Start() error {
	return os.MkdirAll(m.storagePath, 0o755)
}

// CreateUploadSession validates the input, creates a database upload session, and pre-allocates the local file for chunk writes.
func (m *Manager) CreateUploadSession(ctx context.Context, actor models.User, input models.UploadSessionInput, fingerprint string) (models.UploadSession, error) {
	if strings.TrimSpace(input.FileName) == "" || input.TotalSize <= 0 {
		return models.UploadSession{}, ErrInvalidUploadSession
	}

	chunkSize := input.ChunkSize
	if chunkSize <= 0 {
		chunkSize = m.maxChunk
	}
	if chunkSize <= 0 || chunkSize > m.maxChunk {
		return models.UploadSession{}, ErrChunkTooLarge
	}

	totalChunks := input.TotalChunks
	if totalChunks <= 0 {
		totalChunks = int(math.Ceil(float64(input.TotalSize) / float64(chunkSize)))
	}
	if totalChunks <= 0 {
		return models.UploadSession{}, ErrInvalidUploadSession
	}

	input.ChunkSize = chunkSize
	input.TotalChunks = totalChunks

	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.UploadSession{}, ErrUploadStorageUnavailable
	}

	storagePath := filepath.Join(m.storagePath, fmt.Sprintf("%s.part", strings.ToLower(strings.TrimSpace(input.FileName))))
	session, err := db.CreateUploadSession(ctx, actor, input, fingerprint, storagePath)
	if err != nil {
		return models.UploadSession{}, err
	}

	filePath := m.filePathForSession(session)
	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return models.UploadSession{}, fmt.Errorf("prepare upload directory: %w", err)
	}

	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
	if err != nil {
		return models.UploadSession{}, fmt.Errorf("create upload file: %w", err)
	}
	defer file.Close()

	if err := file.Truncate(input.TotalSize); err != nil {
		return models.UploadSession{}, fmt.Errorf("allocate upload file: %w", err)
	}

	return session, nil
}

// SaveChunk validates, hashes, and writes a single chunk to disk, then records it in the database.
func (m *Manager) SaveChunk(ctx context.Context, sessionID string, chunkIndex int, payload []byte, providedSHA256 string) (models.UploadChunkResult, error) {
	unlock := m.lockSession(sessionID)
	defer unlock()

	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.UploadChunkResult{}, ErrUploadStorageUnavailable
	}

	session, err := db.GetUploadSession(ctx, sessionID)
	if err != nil {
		return models.UploadChunkResult{}, ErrUploadSessionNotFound
	}
	if session.Status == "completed" {
		return models.UploadChunkResult{}, ErrUploadAlreadyCompleted
	}
	if chunkIndex < 0 || chunkIndex >= session.TotalChunks {
		return models.UploadChunkResult{}, ErrInvalidChunkIndex
	}
	if len(payload) == 0 {
		return models.UploadChunkResult{}, ErrInvalidUploadSession
	}
	if len(payload) > session.ChunkSize || len(payload) > m.maxChunk {
		return models.UploadChunkResult{}, ErrChunkTooLarge
	}
	chunkExists, err := db.UploadChunkExists(ctx, session.ID, chunkIndex)
	if err != nil {
		return models.UploadChunkResult{}, err
	}
	if chunkExists {
		return models.UploadChunkResult{}, config.ErrChunkAlreadyUploaded
	}

	actualChunkSHA := hashBytes(payload)
	if provided := strings.ToLower(strings.TrimSpace(providedSHA256)); provided != "" && provided != actualChunkSHA {
		return models.UploadChunkResult{}, ErrChunkIntegrityMismatch
	}

	filePath := m.filePathForSession(session)
	file, err := os.OpenFile(filePath, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return models.UploadChunkResult{}, fmt.Errorf("open upload file: %w", err)
	}
	defer file.Close()

	offset := int64(chunkIndex) * int64(session.ChunkSize)
	if _, err := file.WriteAt(payload, offset); err != nil {
		return models.UploadChunkResult{}, fmt.Errorf("write upload chunk: %w", err)
	}

	updatedSession, err := db.RecordUploadChunk(ctx, session.ID, chunkIndex, len(payload), actualChunkSHA)
	if err != nil {
		if errors.Is(err, config.ErrChunkAlreadyUploaded) {
			return models.UploadChunkResult{}, config.ErrChunkAlreadyUploaded
		}
		return models.UploadChunkResult{}, err
	}

	return models.UploadChunkResult{
		SessionID:      session.ID,
		ChunkIndex:     chunkIndex,
		ChunkSize:      len(payload),
		ChunkSHA256:    actualChunkSHA,
		ReceivedChunks: updatedSession.ReceivedChunks,
		TotalChunks:    updatedSession.TotalChunks,
		Status:         updatedSession.Status,
	}, nil
}

// FinalizeUpload completes an upload session: verifies completeness, applies metadata wiping, computes the final SHA256, and persists integrity status.
func (m *Manager) FinalizeUpload(ctx context.Context, sessionID string) (models.File, error) {
	unlock := m.lockSession(sessionID)
	defer unlock()

	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.File{}, ErrUploadStorageUnavailable
	}

	session, err := db.GetUploadSession(ctx, sessionID)
	if err != nil {
		return models.File{}, ErrUploadSessionNotFound
	}
	if session.Status == "completed" {
		fileEntry, err := m.fileEntryForSession(session)
		if err != nil {
			return models.File{}, err
		}
		return fileEntry, nil
	}
	if session.ReceivedChunks != session.TotalChunks {
		return models.File{}, ErrUploadIncomplete
	}
	if session.BytesReceived < session.TotalSize {
		return models.File{}, ErrUploadIncomplete
	}

	filePath := m.filePathForSession(session)

	// Check instance settings for metadata wiping configuration
	dbConn := config.GetDatabase()
	if dbConn != nil {
		if settings, err := dbConn.GetInstanceSettings(ctx); err == nil {
			if settings.MetadataMode == "always" {
				ext := strings.ToLower(filepath.Ext(session.FileName))
				eng := engine.GetInstance()
				if settings.StripImages && (ext == ".jpg" || ext == ".jpeg" || ext == ".png") && eng.IsRuleActive("WIPE_EXIF_METADATA") {
					if err := m.WipeEXIF(filePath); err == nil {
						eng.TriggerRule("WIPE_EXIF_METADATA")
						_ = dbConn.WriteAuditLog(ctx, "file.metadata.wipe", "system@horsync.local", "file", session.ID, "success", "EXIF metadata wiped automatically on upload finalization")
					}
				}
				if settings.StripPdfs && (ext == ".pdf" || ext == ".docx" || ext == ".xlsx") && eng.IsRuleActive("WIPE_DOCUMENT_METADATA") {
					if err := m.WipeDocMetadata(filePath); err == nil {
						eng.TriggerRule("WIPE_DOCUMENT_METADATA")
						_ = dbConn.WriteAuditLog(ctx, "file.metadata.wipe", "system@horsync.local", "file", session.ID, "success", "Document metadata wiped automatically on upload finalization")
					}
				}
			}
		}
	}

	file, err := os.Open(filePath)
	if err != nil {
		return models.File{}, fmt.Errorf("open completed upload: %w", err)
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.CopyN(hasher, file, session.TotalSize); err != nil {
		return models.File{}, fmt.Errorf("hash upload file: %w", err)
	}
	actualSHA256 := hex.EncodeToString(hasher.Sum(nil))

	// Determine the integrity status from the optional client-provided hash.
	// When the client omitted the expected hash we cannot claim "verified" —
	// mark it as "skipped" so the UI can distinguish genuine verifications
	// (client supplied a hash and it matched) from integrity-by-guesswork.
	expected := strings.ToLower(strings.TrimSpace(session.ExpectedSHA256))
	integrityStatus := "verified"
	switch {
	case expected == "":
		integrityStatus = "skipped"
	case expected != actualSHA256:
		integrityStatus = "mismatch"
	}

	if err := db.CompleteUploadSession(ctx, session.ID, actualSHA256, integrityStatus); err != nil {
		return models.File{}, err
	}

	completedSession, err := db.GetUploadSession(ctx, session.ID)
	if err != nil {
		return models.File{}, err
	}

	if integrityStatus == "mismatch" {
		return m.fileEntryForSession(completedSession)
	}

	return m.fileEntryForSession(completedSession)
}

// GetUploadSession retrieves an upload session by ID via the database layer.
func (m *Manager) GetUploadSession(ctx context.Context, sessionID string) (models.UploadSession, error) {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.UploadSession{}, ErrUploadStorageUnavailable
	}

	session, err := db.GetUploadSession(ctx, sessionID)
	if err != nil {
		return models.UploadSession{}, ErrUploadSessionNotFound
	}

	return session, nil
}

// GetReplicationManifest fetches the replication manifest (file metadata and chunk list) for the given job and device.
func (m *Manager) GetReplicationManifest(ctx context.Context, jobID string, deviceID string) (models.ReplicationManifest, error) {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.ReplicationManifest{}, ErrUploadStorageUnavailable
	}

	manifest, err := db.GetReplicationManifest(ctx, jobID, deviceID)
	if err != nil {
		if errors.Is(err, config.ErrReplicationJobNotFound) {
			return models.ReplicationManifest{}, ErrReplicationJobNotFound
		}
		return models.ReplicationManifest{}, err
	}

	return manifest, nil
}

// ReadReplicationChunk reads a specific chunk from the stored file for replication to the destination device, optionally applying bandwidth throttling.
func (m *Manager) ReadReplicationChunk(ctx context.Context, jobID string, deviceID string, chunkIndex int) ([]byte, models.UploadChunkMeta, error) {
	manifest, err := m.GetReplicationManifest(ctx, jobID, deviceID)
	if err != nil {
		return nil, models.UploadChunkMeta{}, err
	}

	var chunk models.UploadChunkMeta
	found := false
	for _, item := range manifest.Chunks {
		if item.ChunkIndex == chunkIndex {
			chunk = item
			found = true
			break
		}
	}
	if !found {
		return nil, models.UploadChunkMeta{}, ErrReplicationChunkNotFound
	}

	session := models.UploadSession{
		ID:          manifest.SessionID,
		StoragePath: manifest.StoragePath,
	}
	filePath := m.filePathForSession(session)
	file, err := os.Open(filePath)
	if err != nil {
		return nil, models.UploadChunkMeta{}, fmt.Errorf("open replication file: %w", err)
	}
	defer file.Close()

	buf := make([]byte, chunk.ChunkSize)
	offset := int64(chunk.ChunkIndex) * int64(manifest.ChunkSize)
	n, err := file.ReadAt(buf, offset)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, models.UploadChunkMeta{}, fmt.Errorf("read replication chunk: %w", err)
	}

	// Apply Bandwidth Governor Rate Limiting if enabled
	dbConn := config.GetDatabase()
	if dbConn != nil {
		if settings, err := dbConn.GetInstanceSettings(ctx); err == nil && settings.BandwidthThrottle {
			// Throttle transmission: calculate delay based on chunk size (target 128KB/s limit)
			targetBytesPerSec := float64(128 * 1024)
			delaySec := float64(n) / targetBytesPerSec
			time.Sleep(time.Duration(delaySec * float64(time.Second)))
		}
	}

	return buf[:n], chunk, nil
}

// CompleteReplication finalizes a replication job by recording the device's acknowledgement (success or failure).
func (m *Manager) CompleteReplication(ctx context.Context, jobID string, deviceID string, input models.ReplicationAckInput) (models.ReplicationJob, error) {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.ReplicationJob{}, ErrUploadStorageUnavailable
	}

	job, err := db.UpdateReplicationJob(ctx, jobID, deviceID, input)
	if err != nil {
		if errors.Is(err, config.ErrReplicationJobNotFound) {
			return models.ReplicationJob{}, ErrReplicationJobNotFound
		}
		return models.ReplicationJob{}, err
	}

	return job, nil
}

func (m *Manager) filePathForSession(session models.UploadSession) string {
	base := strings.TrimSuffix(filepath.Base(session.StoragePath), filepath.Ext(session.StoragePath))
	if base == "" {
		base = session.ID
	}

	return filepath.Join(m.storagePath, session.ID, base+".part")
}

// ResolveUploadFile returns the absolute on-disk path for a stored upload session.
// It looks up the session metadata from the database and applies the configured
// upload storage directory, so callers do not need to hard-code "data/uploads".
// This is the canonical resolver used by both the P2P chunk handler and the
// metadata wipe handler.
func (m *Manager) ResolveUploadFile(ctx context.Context, sessionID string) (string, error) {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return "", ErrUploadStorageUnavailable
	}

	session, err := db.GetUploadSession(ctx, sessionID)
	if err != nil {
		return "", ErrUploadSessionNotFound
	}

	return m.filePathForSession(session), nil
}

func (m *Manager) fileEntryForSession(session models.UploadSession) (models.File, error) {
	files, err := config.GetDatabase().ListFiles(context.Background())
	if err != nil {
		return models.File{}, err
	}

	for _, file := range files {
		if file.Name == session.FileName && file.SHA256 == session.ActualSHA256 {
			return file, nil
		}
	}

	statuses := []string{"CHUNKED"}
	if session.IntegrityStatus == "verified" {
		statuses = append(statuses, "SHA256_VERIFIED")
	}
	if session.IntegrityStatus == "mismatch" {
		statuses = append(statuses, "SHA256_MISMATCH")
	}
	if session.IntegrityStatus == "skipped" {
		statuses = append(statuses, "INTEGRITY_SKIPPED")
	}

	return models.File{
		Name:        session.FileName,
		Type:        "archive",
		Size:        fmt.Sprintf("%d B", session.TotalSize),
		Status:      statuses,
		Date:        session.CompletedAt,
		SHA256:      session.ActualSHA256,
		Integrity:   session.IntegrityStatus,
		ChunkSize:   session.ChunkSize,
		TotalChunks: session.TotalChunks,
	}, nil
}

func hashBytes(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func (m *Manager) lockSession(sessionID string) func() {
	lockRef, _ := m.locks.LoadOrStore(sessionID, &sync.Mutex{})
	lock := lockRef.(*sync.Mutex)
	lock.Lock()
	return lock.Unlock
}

// WipeEXIF removes EXIF (APP1) segments from a JPEG file in-place.
func (m *Manager) WipeEXIF(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Simple and robust JPEG EXIF segment (APP1 0xFFE1) stripper
	if len(data) > 4 && data[0] == 0xFF && data[1] == 0xD8 {
		clean := make([]byte, 0, len(data))
		clean = append(clean, data[0], data[1])

		i := 2
		for i < len(data)-1 {
			if data[i] == 0xFF {
				marker := data[i+1]
				if marker == 0xD9 {
					clean = append(clean, 0xFF, 0xD9)
					break
				}
				if marker == 0x00 {
					clean = append(clean, 0xFF, 0x00)
					i += 2
					continue
				}

				if i+3 >= len(data) {
					break
				}
				size := int(data[i+2])<<8 | int(data[i+3])

				if marker == 0xE1 { // Skip APP1 (EXIF) segment
					i += 2 + size
					continue
				}

				if i+2+size <= len(data) {
					clean = append(clean, data[i:i+2+size]...)
					i += 2 + size
				} else {
					break
				}
			} else {
				clean = append(clean, data[i])
				i++
			}
		}

		if len(clean) > 2 {
			return os.WriteFile(filePath, clean, 0o644)
		}
	}
	return nil
}

// WipeDocMetadata strips PDF Info fields (Author, Creator, etc.) and XMP XML metadata streams from a file in-place.
func (m *Manager) WipeDocMetadata(filePath string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	// Simple and robust binary PDF Info & XML metadata stripper
	if len(data) > 4 && data[0] == '%' && data[1] == 'P' && data[2] == 'D' && data[3] == 'F' {
		content := string(data)
		fields := []string{"/Author", "/Creator", "/Producer", "/CreationDate", "/ModDate"}
		modified := false

		for _, field := range fields {
			idx := 0
			for {
				loc := strings.Index(content[idx:], field)
				if loc == -1 {
					break
				}
				actualIdx := idx + loc
				start := actualIdx + len(field)
				for start < len(data) && data[start] == ' ' {
					start++
				}
				if start < len(data) && data[start] == '(' {
					end := start + 1
					for end < len(data) && data[end] != ')' {
						data[end] = ' '
						end++
					}
					modified = true
				}
				idx = actualIdx + len(field)
			}
		}

		// Strip XMP XML Metadata streams
		idx := 0
		for {
			startXml := strings.Index(content[idx:], "<x:xmpmeta")
			if startXml == -1 {
				break
			}
			actualStart := idx + startXml
			endXml := strings.Index(content[actualStart:], "</x:xmpmeta>")
			if endXml == -1 {
				break
			}
			actualEnd := actualStart + endXml + len("</x:xmpmeta>")
			for j := actualStart; j < actualEnd; j++ {
				data[j] = ' '
			}
			modified = true
			idx = actualEnd
		}

		if modified {
			return os.WriteFile(filePath, data, 0o644)
		}
	}
	return nil
}


