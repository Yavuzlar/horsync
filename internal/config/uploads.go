package config

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"horsync/internal/models"
	"horsync/pkg/utils"

	"github.com/google/uuid"
)

var ErrUploadSessionNotFound = errors.New("upload session not found")
var ErrChunkAlreadyUploaded = errors.New("chunk already uploaded")

// CreateUploadSession registers a new file upload session in the database with the provided metadata.
func (db *Database) CreateUploadSession(ctx context.Context, actor models.User, input models.UploadSessionInput, fingerprint string, storagePath string) (models.UploadSession, error) {
	now := time.Now().UTC()
	session := models.UploadSession{
		ID:                      uuid.NewString(),
		FileName:                sanitizeFileName(input.FileName),
		ContentType:             normalizeValue(input.ContentType, "application/octet-stream"),
		TotalSize:               input.TotalSize,
		ChunkSize:               input.ChunkSize,
		TotalChunks:             input.TotalChunks,
		ExpectedSHA256:          strings.ToLower(strings.TrimSpace(input.ExpectedSHA256)),
		IntegrityStatus:         "pending",
		OwnerEmail:              actor.Email,
		SourceDeviceID:          strings.TrimSpace(input.SourceDeviceID),
		SourceDeviceFingerprint: strings.TrimSpace(fingerprint),
		Status:                  "pending",
		StoragePath:             storagePath,
		CreatedAt:               now.Format(time.RFC3339),
		UpdatedAt:               now.Format(time.RFC3339),
	}

	if _, err := db.Pool.Exec(
		ctx,
		`
		INSERT INTO file_upload_sessions (
			id,
			file_name,
			content_type,
			total_size,
			chunk_size,
			total_chunks,
			expected_sha256,
			actual_sha256,
			integrity_status,
			uploaded_by,
			owner_email,
			source_device_id,
			source_device_fingerprint,
			status,
			storage_path,
			received_chunks,
			bytes_received,
			created_at,
			updated_at,
			completed_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, '', $8, $9, $10, $11, $12, $13, $14, 0, 0, $15, $15, NULL
		)
		`,
		session.ID,
		session.FileName,
		session.ContentType,
		session.TotalSize,
		session.ChunkSize,
		session.TotalChunks,
		session.ExpectedSHA256,
		session.IntegrityStatus,
		actor.ID,
		session.OwnerEmail,
		session.SourceDeviceID,
		session.SourceDeviceFingerprint,
		session.Status,
		session.StoragePath,
		now,
	); err != nil {
		return models.UploadSession{}, fmt.Errorf("create upload session: %w", err)
	}

	return session, nil
}

// GetUploadSession retrieves an upload session by its ID, or returns ErrUploadSessionNotFound.
func (db *Database) GetUploadSession(ctx context.Context, sessionID string) (models.UploadSession, error) {
	var (
		session     models.UploadSession
		completedAt *time.Time
		createdAt   time.Time
		updatedAt   time.Time
	)

	err := db.Pool.QueryRow(
		ctx,
		`
		SELECT
			id,
			file_name,
			content_type,
			total_size,
			chunk_size,
			total_chunks,
			expected_sha256,
			actual_sha256,
			integrity_status,
			owner_email,
			source_device_id,
			source_device_fingerprint,
			status,
			storage_path,
			received_chunks,
			bytes_received,
			created_at,
			updated_at,
			completed_at
		FROM file_upload_sessions
		WHERE id = $1
		`,
		strings.TrimSpace(sessionID),
	).Scan(
		&session.ID,
		&session.FileName,
		&session.ContentType,
		&session.TotalSize,
		&session.ChunkSize,
		&session.TotalChunks,
		&session.ExpectedSHA256,
		&session.ActualSHA256,
		&session.IntegrityStatus,
		&session.OwnerEmail,
		&session.SourceDeviceID,
		&session.SourceDeviceFingerprint,
		&session.Status,
		&session.StoragePath,
		&session.ReceivedChunks,
		&session.BytesReceived,
		&createdAt,
		&updatedAt,
		&completedAt,
	)
	if err != nil {
		return models.UploadSession{}, ErrUploadSessionNotFound
	}

	session.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	session.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	if completedAt != nil {
		session.CompletedAt = completedAt.UTC().Format(time.RFC3339)
	}

	return session, nil
}

// RecordUploadChunk persists a received chunk's metadata and updates the session progress atomically.
func (db *Database) RecordUploadChunk(ctx context.Context, sessionID string, chunkIndex int, chunkSize int, chunkSHA256 string) (models.UploadSession, error) {
	tx, err := db.Pool.Begin(ctx)
	if err != nil {
		return models.UploadSession{}, fmt.Errorf("begin upload chunk tx: %w", err)
	}
	defer tx.Rollback(ctx)

	var existingHash string
	err = tx.QueryRow(
		ctx,
		`
		SELECT chunk_sha256
		FROM file_upload_chunks
		WHERE session_id = $1 AND chunk_index = $2
		`,
		sessionID,
		chunkIndex,
	).Scan(&existingHash)
	if err == nil {
		return models.UploadSession{}, ErrChunkAlreadyUploaded
	}

	now := time.Now().UTC()
	if _, err := tx.Exec(
		ctx,
		`
		INSERT INTO file_upload_chunks (
			session_id,
			chunk_index,
			chunk_size,
			chunk_sha256,
			received_at
		)
		VALUES ($1, $2, $3, $4, $5)
		`,
		sessionID,
		chunkIndex,
		chunkSize,
		chunkSHA256,
		now,
	); err != nil {
		return models.UploadSession{}, fmt.Errorf("insert upload chunk: %w", err)
	}

	if _, err := tx.Exec(
		ctx,
		`
		UPDATE file_upload_sessions
		SET
			received_chunks = received_chunks + 1,
			bytes_received = bytes_received + $2,
			updated_at = $3
		WHERE id = $1
		`,
		sessionID,
		chunkSize,
		now,
	); err != nil {
		return models.UploadSession{}, fmt.Errorf("update upload session progress: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return models.UploadSession{}, fmt.Errorf("commit upload chunk tx: %w", err)
	}

	return db.GetUploadSession(ctx, sessionID)
}

// UploadChunkExists checks whether a specific chunk has already been received for the given upload session.
func (db *Database) UploadChunkExists(ctx context.Context, sessionID string, chunkIndex int) (bool, error) {
	var exists bool
	err := db.Pool.QueryRow(
		ctx,
		`
		SELECT EXISTS(
			SELECT 1
			FROM file_upload_chunks
			WHERE session_id = $1 AND chunk_index = $2
		)
		`,
		sessionID,
		chunkIndex,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("check upload chunk: %w", err)
	}

	return exists, nil
}

// CompleteUploadSession marks an upload session as completed with the computed SHA256 and integrity status.
func (db *Database) CompleteUploadSession(ctx context.Context, sessionID string, actualSHA256 string, integrityStatus string) error {
	status := "completed"
	completedAt := time.Now().UTC()
	if _, err := db.Pool.Exec(
		ctx,
		`
		UPDATE file_upload_sessions
		SET
			actual_sha256 = $2,
			integrity_status = $3,
			status = $4,
			updated_at = $5,
			completed_at = $5
		WHERE id = $1
		`,
		sessionID,
		strings.ToLower(strings.TrimSpace(actualSHA256)),
		integrityStatus,
		status,
		completedAt,
	); err != nil {
		return fmt.Errorf("complete upload session: %w", err)
	}

	return nil
}

// ListFiles returns all completed file upload sessions enriched with file type, size, and metadata status.
func (db *Database) ListFiles(ctx context.Context) ([]models.File, error) {
	rows, err := db.Pool.Query(
		ctx,
		`
		SELECT
			id,
			file_name,
			total_size,
			content_type,
			actual_sha256,
			integrity_status,
			chunk_size,
			total_chunks,
			created_at
		FROM file_upload_sessions
		WHERE status = 'completed'
		ORDER BY created_at DESC
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	defer rows.Close()

	result := make([]models.File, 0)
	index := 1
	for rows.Next() {
		var (
			sessionID       string
			fileName        string
			totalSize       int64
			contentType     string
			actualSHA256    string
			integrityStatus string
			chunkSize       int
			totalChunks     int
			createdAt       time.Time
		)

		if err := rows.Scan(
			&sessionID,
			&fileName,
			&totalSize,
			&contentType,
			&actualSHA256,
			&integrityStatus,
			&chunkSize,
			&totalChunks,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan file upload session: %w", err)
		}

		statuses := []string{"CHUNKED"}
		if integrityStatus == "verified" {
			statuses = append(statuses, "SHA256_VERIFIED")
		} else if integrityStatus == "mismatch" {
			statuses = append(statuses, "SHA256_MISMATCH")
		}

		// Dynamically check for sensitive metadata
		ext := strings.ToLower(filepath.Ext(fileName))
		if ext == ".jpg" || ext == ".jpeg" || ext == ".png" || ext == ".pdf" || ext == ".docx" || ext == ".xlsx" {
			if HasMetadataOnDisk(sessionID, fileName) {
				statuses = append(statuses, "METADATA_WARNING")
			} else {
				// Query if this specific session has a successful metadata wipe audit log
				var wasWiped bool
				dbErr := db.Pool.QueryRow(ctx, `
					SELECT EXISTS(
						SELECT 1 FROM audit_logs 
						WHERE target_id = $1 AND action = 'file.metadata.wipe' AND status = 'success'
					)
				`, sessionID).Scan(&wasWiped)
				if dbErr == nil && wasWiped {
					statuses = append(statuses, "METADATA_CLEANED")
				}
			}
		}

		result = append(result, models.File{
			ID:          index,
			Name:        fileName,
			Type:        inferFileType(fileName, contentType),
			Size:        utils.FormatBytes(totalSize),
			Status:      statuses,
			Date:        utils.FormatRelativeTime(createdAt),
			SHA256:      actualSHA256,
			Integrity:   integrityStatus,
			ChunkSize:   chunkSize,
			TotalChunks: totalChunks,
		})
		index++
	}

	return result, rows.Err()
}

func sanitizeFileName(name string) string {
	cleaned := strings.TrimSpace(filepath.Base(name))
	if cleaned == "" || cleaned == "." || cleaned == string(filepath.Separator) {
		return "unnamed.bin"
	}

	return cleaned
}

func inferFileType(fileName string, contentType string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(fileName), "."))
	switch ext {
	case "pdf":
		return "pdf"
	case "zip", "rar", "7z", "tar", "gz":
		return "archive"
	case "mp4", "mov", "mkv":
		return "video"
	case "png", "jpg", "jpeg", "webp", "fig":
		return "design"
	}

	if strings.HasPrefix(strings.ToLower(contentType), "video/") {
		return "video"
	}
	if strings.HasPrefix(strings.ToLower(contentType), "image/") {
		return "design"
	}

	return "archive"
}

// HasMetadataOnDisk checks if a stored file has detectable sensitive metadata (EXIF, PDF author, XMP).
func HasMetadataOnDisk(sessionID string, fileName string) bool {
	filePath := filepath.Join("data/uploads", sessionID, strings.TrimSuffix(fileName, filepath.Ext(fileName))+".part")
	if _, err := os.Stat(filePath); err != nil {
		return false
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	if ext == ".jpg" || ext == ".jpeg" {
		if len(data) > 4 && data[0] == 0xFF && data[1] == 0xD8 {
			i := 2
			for i < len(data)-3 {
				if data[i] == 0xFF {
					marker := data[i+1]
					if marker == 0xE1 {
						return true
					}
					if marker == 0xD9 || marker == 0x00 {
						break
					}
					size := int(data[i+2])<<8 | int(data[i+3])
					i += 2 + size
				} else {
					i++
				}
			}
		}
	} else if ext == ".pdf" {
		content := string(data)
		if strings.Contains(content, "/Author") || strings.Contains(content, "/Creator") || strings.Contains(content, "<x:xmpmeta") {
			// Double check if they are actually filled with content and not just empty spaces
			if strings.Contains(content, "/Author (") && !strings.Contains(content, "/Author (   ") {
				return true
			}
			if strings.Contains(content, "<x:xmpmeta") && strings.Contains(content, "xmlns:x") {
				return true
			}
		}
	}
	return false
}
