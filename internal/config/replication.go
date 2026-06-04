package config

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"horsync/internal/models"

	"github.com/google/uuid"
)

var ErrReplicationJobNotFound = errors.New("replication job not found")
var ErrDeviceAgentUnauthorized = errors.New("device agent unauthorized")

func (db *Database) ValidateDeviceAgent(ctx context.Context, auth models.DeviceAgentAuth) (models.Node, error) {
	var (
		device          models.Node
		loadPercent     int
		uptimeSeconds   int64
		lastSeenAt      *time.Time
		createdAt       time.Time
		approvedAt      *time.Time
		fingerprintHash string
	)

	err := db.Pool.QueryRow(
		ctx,
		`
		SELECT
			device_id,
			name,
			type,
			location,
			status,
			ip,
			owner_email,
			sync_mode,
			load_percent,
			uptime_seconds,
			last_seen_at,
			created_at,
			approved_at,
			fingerprint_hash
		FROM devices
		WHERE device_id = $1
			AND device_secret_hash = $2
			AND status = 'active'
		`,
		strings.TrimSpace(auth.DeviceID),
		hashToken(strings.TrimSpace(auth.DeviceSecret)),
	).Scan(
		&device.ID,
		&device.Name,
		&device.Type,
		&device.Location,
		&device.Status,
		&device.IP,
		&device.OwnerEmail,
		&device.SyncMode,
		&loadPercent,
		&uptimeSeconds,
		&lastSeenAt,
		&createdAt,
		&approvedAt,
		&fingerprintHash,
	)
	if err != nil {
		return models.Node{}, ErrDeviceAgentUnauthorized
	}

	device.Load = fmt.Sprintf("%d%%", loadPercent)
	device.Uptime = replicationFormatUptime(uptimeSeconds)
	device.LastSeen = replicationFormatLastSeen(lastSeenAt)
	device.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	if approvedAt != nil {
		device.ApprovedAt = approvedAt.UTC().Format(time.RFC3339)
	}
	device.FingerprintPreview = replicationPreviewFingerprint(fingerprintHash)
	return device, nil
}

func (db *Database) CreateReplicationJobs(ctx context.Context, uploadSessionID string, sourceDeviceID string) ([]models.ReplicationJob, error) {
	rows, err := db.Pool.Query(
		ctx,
		`
		SELECT device_id
		FROM devices
		WHERE status = 'active'
			AND device_id <> $1
		ORDER BY created_at DESC
		`,
		strings.TrimSpace(sourceDeviceID),
	)
	if err != nil {
		return nil, fmt.Errorf("list replication targets: %w", err)
	}
	defer rows.Close()

	now := time.Now().UTC()
	result := make([]models.ReplicationJob, 0)
	for rows.Next() {
		var deviceID string
		if err := rows.Scan(&deviceID); err != nil {
			return nil, fmt.Errorf("scan replication target: %w", err)
		}

		job := models.ReplicationJob{
			ID:              uuid.NewString(),
			UploadSessionID: uploadSessionID,
			DeviceID:        deviceID,
			Status:          "queued",
			CreatedAt:       now.Format(time.RFC3339),
			UpdatedAt:       now.Format(time.RFC3339),
		}

		if _, err := db.Pool.Exec(
			ctx,
			`
			INSERT INTO replication_jobs (
				id,
				upload_session_id,
				device_id,
				status,
				verified_sha256,
				last_error,
				created_at,
				updated_at,
				completed_at
			)
			VALUES ($1, $2, $3, $4, '', '', $5, $5, NULL)
			ON CONFLICT (upload_session_id, device_id) DO NOTHING
			`,
			job.ID,
			job.UploadSessionID,
			job.DeviceID,
			job.Status,
			now,
		); err != nil {
			return nil, fmt.Errorf("create replication job: %w", err)
		}

		result = append(result, job)
	}

	return result, rows.Err()
}

func (db *Database) ListReplicationJobsForDevice(ctx context.Context, deviceID string) ([]models.ReplicationJob, error) {
	rows, err := db.Pool.Query(
		ctx,
		`
		SELECT id, upload_session_id, device_id, status, verified_sha256, last_error, created_at, updated_at, completed_at
		FROM replication_jobs
		WHERE device_id = $1
			AND status IN ('queued', 'verifying')
		ORDER BY created_at ASC
		`,
		strings.TrimSpace(deviceID),
	)
	if err != nil {
		return nil, fmt.Errorf("list replication jobs: %w", err)
	}
	defer rows.Close()

	result := make([]models.ReplicationJob, 0)
	for rows.Next() {
		item, err := scanReplicationJob(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, item)
	}

	return result, rows.Err()
}

func (db *Database) GetReplicationManifest(ctx context.Context, jobID string, deviceID string) (models.ReplicationManifest, error) {
	var manifest models.ReplicationManifest

	err := db.Pool.QueryRow(
		ctx,
		`
		SELECT
			r.id,
			s.id,
			s.file_name,
			s.content_type,
			s.total_size,
			s.chunk_size,
			s.total_chunks,
			s.actual_sha256,
			s.source_device_id,
			s.source_device_fingerprint,
			s.storage_path
		FROM replication_jobs r
		INNER JOIN file_upload_sessions s ON s.id = r.upload_session_id
		WHERE r.id = $1
			AND r.device_id = $2
		`,
		strings.TrimSpace(jobID),
		strings.TrimSpace(deviceID),
	).Scan(
		&manifest.JobID,
		&manifest.SessionID,
		&manifest.FileName,
		&manifest.ContentType,
		&manifest.TotalSize,
		&manifest.ChunkSize,
		&manifest.TotalChunks,
		&manifest.ExpectedSHA256,
		&manifest.SourceDeviceID,
		&manifest.SourceFingerprint,
		&manifest.StoragePath,
	)
	if err != nil {
		return models.ReplicationManifest{}, ErrReplicationJobNotFound
	}

	rows, err := db.Pool.Query(
		ctx,
		`
		SELECT chunk_index, chunk_size, chunk_sha256
		FROM file_upload_chunks
		WHERE session_id = $1
		ORDER BY chunk_index ASC
		`,
		manifest.SessionID,
	)
	if err != nil {
		return models.ReplicationManifest{}, fmt.Errorf("list replication chunks: %w", err)
	}
	defer rows.Close()

	manifest.Chunks = make([]models.UploadChunkMeta, 0, manifest.TotalChunks)
	for rows.Next() {
		var item models.UploadChunkMeta
		if err := rows.Scan(&item.ChunkIndex, &item.ChunkSize, &item.ChunkSHA256); err != nil {
			return models.ReplicationManifest{}, fmt.Errorf("scan replication chunk: %w", err)
		}
		manifest.Chunks = append(manifest.Chunks, item)
	}

	if err := rows.Err(); err != nil {
		return models.ReplicationManifest{}, err
	}

	_, _ = db.Pool.Exec(
		ctx,
		`
		UPDATE replication_jobs
		SET status = 'verifying', updated_at = $2
		WHERE id = $1 AND status = 'queued'
		`,
		manifest.JobID,
		time.Now().UTC(),
	)

	return manifest, nil
}

func (db *Database) UpdateReplicationJob(ctx context.Context, jobID string, deviceID string, input models.ReplicationAckInput) (models.ReplicationJob, error) {
	status := strings.TrimSpace(strings.ToLower(input.Status))
	if status == "" {
		status = "failed"
	}

	completedAt := time.Now().UTC()
	commandTag, err := db.Pool.Exec(
		ctx,
		`
		UPDATE replication_jobs
		SET
			status = $3,
			verified_sha256 = $4,
			last_error = $5,
			updated_at = $6,
			completed_at = $6
		WHERE id = $1 AND device_id = $2
		`,
		strings.TrimSpace(jobID),
		strings.TrimSpace(deviceID),
		status,
		strings.ToLower(strings.TrimSpace(input.VerifiedSHA256)),
		strings.TrimSpace(input.LastError),
		completedAt,
	)
	if err != nil {
		return models.ReplicationJob{}, fmt.Errorf("update replication job: %w", err)
	}
	if commandTag.RowsAffected() == 0 {
		return models.ReplicationJob{}, ErrReplicationJobNotFound
	}

	return db.GetReplicationJob(ctx, jobID, deviceID)
}

func (db *Database) GetReplicationJob(ctx context.Context, jobID string, deviceID string) (models.ReplicationJob, error) {
	rows, err := db.Pool.Query(
		ctx,
		`
		SELECT id, upload_session_id, device_id, status, verified_sha256, last_error, created_at, updated_at, completed_at
		FROM replication_jobs
		WHERE id = $1 AND device_id = $2
		LIMIT 1
		`,
		strings.TrimSpace(jobID),
		strings.TrimSpace(deviceID),
	)
	if err != nil {
		return models.ReplicationJob{}, fmt.Errorf("get replication job: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return models.ReplicationJob{}, ErrReplicationJobNotFound
	}

	return scanReplicationJob(rows)
}

func scanReplicationJob(scanner interface {
	Scan(dest ...any) error
}) (models.ReplicationJob, error) {
	var (
		item        models.ReplicationJob
		createdAt   time.Time
		updatedAt   time.Time
		completedAt *time.Time
	)

	if err := scanner.Scan(
		&item.ID,
		&item.UploadSessionID,
		&item.DeviceID,
		&item.Status,
		&item.VerifiedSHA256,
		&item.LastError,
		&createdAt,
		&updatedAt,
		&completedAt,
	); err != nil {
		return models.ReplicationJob{}, fmt.Errorf("scan replication job: %w", err)
	}

	item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	item.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	if completedAt != nil {
		item.CompletedAt = completedAt.UTC().Format(time.RFC3339)
	}

	return item, nil
}

func replicationFormatUptime(totalSeconds int64) string {
	if totalSeconds <= 0 {
		return "0D 00H"
	}

	duration := time.Duration(totalSeconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour

	return fmt.Sprintf("%dD %02dH", days, hours)
}

func replicationFormatLastSeen(value *time.Time) string {
	if value == nil {
		return "Awaiting approval"
	}

	elapsed := time.Since(value.UTC())
	switch {
	case elapsed < time.Minute:
		return "Just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
}

func replicationPreviewFingerprint(value string) string {
	if len(value) <= 12 {
		return value
	}

	return value[:12]
}

