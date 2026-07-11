package config

import (
	"context"
	"fmt"
	"time"

	"horsync/internal/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type Database struct {
	Pool *pgxpool.Pool
}

var currentDatabase *Database

// InitDatabase creates a new connection pool to the PostgreSQL database and verifies connectivity.
func InitDatabase(ctx context.Context, databaseURL string) (*Database, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	db := &Database{Pool: pool}
	currentDatabase = db
	return db, nil
}

// GetDatabase returns the global Database instance previously created by InitDatabase.
func GetDatabase() *Database {
	return currentDatabase
}

// Close shuts down the underlying PostgreSQL connection pool.
func (db *Database) Close() {
	if db == nil || db.Pool == nil {
		return
	}

	db.Pool.Close()
}

// Migrate runs all database schema migrations and seeds initial data (admin user, settings, automation rules).
func (db *Database) Migrate(ctx context.Context) error {
	statements := []string{
		`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			name TEXT NOT NULL,
			role TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);
			`,
		`
		CREATE TABLE IF NOT EXISTS auth_sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
			token_hash TEXT NOT NULL UNIQUE,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);
			`,
		`
		CREATE TABLE IF NOT EXISTS device_enrollments (
			id TEXT PRIMARY KEY,
			token_hash TEXT NOT NULL UNIQUE,
			token_preview TEXT NOT NULL,
			label TEXT NOT NULL,
			device_type TEXT NOT NULL,
			location TEXT NOT NULL,
			owner_email TEXT NOT NULL,
			sync_mode TEXT NOT NULL,
			created_by TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
			status TEXT NOT NULL,
			expires_at TIMESTAMPTZ NOT NULL,
			created_at TIMESTAMPTZ NOT NULL,
			registered_device_id TEXT
		);
			`,
		`
		CREATE TABLE IF NOT EXISTS devices (
			id TEXT PRIMARY KEY,
			device_id TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			type TEXT NOT NULL,
			location TEXT NOT NULL,
			ip TEXT NOT NULL,
			owner_email TEXT NOT NULL,
			status TEXT NOT NULL,
			sync_mode TEXT NOT NULL,
			enrollment_id TEXT REFERENCES device_enrollments(id) ON DELETE SET NULL,
			device_secret_hash TEXT NOT NULL DEFAULT '',
			load_percent INTEGER NOT NULL DEFAULT 0,
			uptime_seconds BIGINT NOT NULL DEFAULT 0,
			last_seen_at TIMESTAMPTZ,
			approved_at TIMESTAMPTZ,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL
		);
			`,
		`
		CREATE INDEX IF NOT EXISTS idx_devices_status
		ON devices(status);
			`,
		`
		ALTER TABLE devices
		ADD COLUMN IF NOT EXISTS enrollment_id TEXT;
			`,
		`
		ALTER TABLE devices
		ADD COLUMN IF NOT EXISTS device_secret_hash TEXT NOT NULL DEFAULT '';
			`,
		`
		ALTER TABLE devices
		ADD COLUMN IF NOT EXISTS fingerprint_hash TEXT NOT NULL DEFAULT '';
			`,
		`
		CREATE TABLE IF NOT EXISTS instance_settings (
			id BOOLEAN PRIMARY KEY DEFAULT TRUE,
			node_name TEXT NOT NULL,
			maintainer_email TEXT NOT NULL,
			smart_delta_sync BOOLEAN NOT NULL DEFAULT TRUE,
			bandwidth_throttle BOOLEAN NOT NULL DEFAULT FALSE,
			updated_at TIMESTAMPTZ NOT NULL
		);
			`,
		`
		CREATE TABLE IF NOT EXISTS audit_logs (
			id TEXT PRIMARY KEY,
			action TEXT NOT NULL,
			actor TEXT NOT NULL,
			target_type TEXT NOT NULL,
			target_id TEXT NOT NULL,
			status TEXT NOT NULL,
			message TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL
		);
			`,
		`
		CREATE TABLE IF NOT EXISTS file_upload_sessions (
			id TEXT PRIMARY KEY,
			file_name TEXT NOT NULL,
			content_type TEXT NOT NULL,
			total_size BIGINT NOT NULL,
			chunk_size INTEGER NOT NULL,
			total_chunks INTEGER NOT NULL,
			expected_sha256 TEXT NOT NULL DEFAULT '',
			actual_sha256 TEXT NOT NULL DEFAULT '',
			integrity_status TEXT NOT NULL DEFAULT 'pending',
			uploaded_by TEXT NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
			owner_email TEXT NOT NULL,
			source_device_id TEXT NOT NULL DEFAULT '',
			source_device_fingerprint TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			storage_path TEXT NOT NULL,
			received_chunks INTEGER NOT NULL DEFAULT 0,
			bytes_received BIGINT NOT NULL DEFAULT 0,
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ
		);
			`,
		`
		CREATE TABLE IF NOT EXISTS file_upload_chunks (
			session_id TEXT NOT NULL REFERENCES file_upload_sessions(id) ON DELETE CASCADE,
			chunk_index INTEGER NOT NULL,
			chunk_size INTEGER NOT NULL,
			chunk_sha256 TEXT NOT NULL,
			received_at TIMESTAMPTZ NOT NULL,
			PRIMARY KEY (session_id, chunk_index)
		);
			`,
		`
		CREATE INDEX IF NOT EXISTS idx_file_upload_sessions_status
		ON file_upload_sessions(status, created_at DESC);
			`,
		`
		CREATE TABLE IF NOT EXISTS replication_jobs (
			id TEXT PRIMARY KEY,
			upload_session_id TEXT NOT NULL REFERENCES file_upload_sessions(id) ON DELETE CASCADE,
			device_id TEXT NOT NULL REFERENCES devices(device_id) ON DELETE CASCADE,
			status TEXT NOT NULL,
			verified_sha256 TEXT NOT NULL DEFAULT '',
			last_error TEXT NOT NULL DEFAULT '',
			created_at TIMESTAMPTZ NOT NULL,
			updated_at TIMESTAMPTZ NOT NULL,
			completed_at TIMESTAMPTZ,
			UNIQUE (upload_session_id, device_id)
		);
			`,
		`
		CREATE INDEX IF NOT EXISTS idx_replication_jobs_device_status
		ON replication_jobs(device_id, status, created_at DESC);
			`,
		`
		ALTER TABLE instance_settings
		ADD COLUMN IF NOT EXISTS p2p_strict_approval BOOLEAN NOT NULL DEFAULT FALSE;
			`,
		`
		ALTER TABLE instance_settings
		ADD COLUMN IF NOT EXISTS metadata_mode TEXT NOT NULL DEFAULT 'always';
			`,
		`
		ALTER TABLE instance_settings
		ADD COLUMN IF NOT EXISTS strip_images BOOLEAN NOT NULL DEFAULT TRUE;
			`,
		`
		ALTER TABLE instance_settings
		ADD COLUMN IF NOT EXISTS strip_pdfs BOOLEAN NOT NULL DEFAULT TRUE;
			`,
			`
			CREATE TABLE IF NOT EXISTS automation_rules (
				id SERIAL PRIMARY KEY,
				name TEXT NOT NULL UNIQUE,
				description TEXT NOT NULL DEFAULT '',
				active BOOLEAN NOT NULL DEFAULT TRUE,
				total_runs INTEGER NOT NULL DEFAULT 0,
				last_triggered_at TEXT NOT NULL DEFAULT 'Not triggered yet',
				created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
				updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
			);
			`,
	}

	for _, statement := range statements {
		if _, err := db.Pool.Exec(ctx, statement); err != nil {
			return fmt.Errorf("run migration: %w", err)
		}
	}

	if err := db.seedInstanceSettings(ctx); err != nil {
		return err
	}

	if err := db.seedAutomationRules(ctx); err != nil {
		return err
	}

	if err := db.seedAdminUser(ctx); err != nil {
		return err
	}

	if err := db.cleanupLegacySeedDevices(ctx); err != nil {
		return err
	}

	return nil
}

func (db *Database) seedAdminUser(ctx context.Context) error {
	var count int
	if err := db.Pool.QueryRow(ctx, `SELECT COUNT(*) FROM users`).Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}

	if count > 0 {
		return nil
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("admin12345"), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash admin password: %w", err)
	}

	_, err = db.Pool.Exec(
		ctx,
		`
		INSERT INTO users (
			id,
			email,
			password_hash,
			name,
			role,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6)
			`,
		uuid.NewString(),
		"admin@horsync.local",
		string(passwordHash),
		"Platform Admin",
		"admin",
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("seed admin user: %w", err)
	}

	return nil
}

// GetInstanceSettings retrieves the current instance-level configuration settings.
func (db *Database) GetInstanceSettings(ctx context.Context) (models.InstanceSettings, error) {
	var settings models.InstanceSettings
	var updatedAt time.Time

	err := db.Pool.QueryRow(
		ctx,
		`
		SELECT
			node_name,
			maintainer_email,
			smart_delta_sync,
			bandwidth_throttle,
			p2p_strict_approval,
			metadata_mode,
			strip_images,
			strip_pdfs,
			updated_at
		FROM instance_settings
		WHERE id = TRUE
			`,
	).Scan(
		&settings.NodeName,
		&settings.MaintainerEmail,
		&settings.SmartDeltaSync,
		&settings.BandwidthThrottle,
		&settings.P2PStrictApproval,
		&settings.MetadataMode,
		&settings.StripImages,
		&settings.StripPdfs,
		&updatedAt,
	)
	if err != nil {
		return models.InstanceSettings{}, fmt.Errorf("get instance settings: %w", err)
	}

	settings.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return settings, nil
}

// UpdateInstanceSettings persists the provided instance configuration and returns the updated settings.
func (db *Database) UpdateInstanceSettings(ctx context.Context, input models.InstanceSettings) (models.InstanceSettings, error) {
	updatedAt := time.Now().UTC()

	_, err := db.Pool.Exec(
		ctx,
		`
		UPDATE instance_settings
		SET
			node_name = $1,
			maintainer_email = $2,
			smart_delta_sync = $3,
			bandwidth_throttle = $4,
			p2p_strict_approval = $5,
			metadata_mode = $6,
			strip_images = $7,
			strip_pdfs = $8,
			updated_at = $9
		WHERE id = TRUE
			`,
		input.NodeName,
		input.MaintainerEmail,
		input.SmartDeltaSync,
		input.BandwidthThrottle,
		input.P2PStrictApproval,
		input.MetadataMode,
		input.StripImages,
		input.StripPdfs,
		updatedAt,
	)
	if err != nil {
		return models.InstanceSettings{}, fmt.Errorf("update instance settings: %w", err)
	}

	input.UpdatedAt = updatedAt.Format(time.RFC3339)
	return input, nil
}

func (db *Database) seedInstanceSettings(ctx context.Context) error {
	_, err := db.Pool.Exec(
		ctx,
		`
		INSERT INTO instance_settings (
			id,
			node_name,
			maintainer_email,
			smart_delta_sync,
			bandwidth_throttle,
			p2p_strict_approval,
			metadata_mode,
			strip_images,
			strip_pdfs,
			updated_at
		)
		VALUES (TRUE, $1, $2, $3, $4, $5, 'always', TRUE, TRUE, $6)
		ON CONFLICT (id) DO NOTHING
			`,
		"HORSYNC_CONTROL_PLANE",
		"maintainer@horsync.org",
		true,
		false,
		false,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("seed instance settings: %w", err)
	}

	return nil
}


func (db *Database) seedAutomationRules(ctx context.Context) error {
	_, err := db.Pool.Exec(ctx, `
		INSERT INTO automation_rules (name, description, active, total_runs, last_triggered_at)
		VALUES
			('AUTO_ENCRYPT_FINANCIALS', 'Encrypt matching PDF files when file reporting is integrated.', TRUE, 0, 'Not triggered yet'),
			('WIPE_EXIF_METADATA', 'Strip image metadata before replication when uploads are enabled.', TRUE, 0, 'Not triggered yet'),
			('COLD_STORAGE_ARCHIVE', 'Archive inactive files after long retention windows.', FALSE, 0, 'Not triggered yet'),
			('INSTANT_SYNC_PRIORITY', 'Prioritize low-latency sync jobs for small files.', TRUE, 0, 'Not triggered yet'),
			('WIPE_DOCUMENT_METADATA', 'Strip author, creation software, and XMP metadata streams from PDF and office documents before replication.', TRUE, 0, 'Not triggered yet')
		ON CONFLICT (name) DO NOTHING
	`)
	if err != nil {
		return fmt.Errorf("seed automation rules: %w", err)
	}

	return nil
}

func (db *Database) cleanupLegacySeedDevices(ctx context.Context) error {
	_, err := db.Pool.Exec(
		ctx,
		`
		DELETE FROM devices
		WHERE device_id IN ('YVS-CORE-01', 'YVS-DESK-07', 'YVS-LPT-12')
			AND enrollment_id IS NULL
			AND owner_email IN (
				'ops@horsync.org',
				'design@horsync.org',
				'field@horsync.org'
			)
			`,
	)
	if err != nil {
		return fmt.Errorf("cleanup legacy seed devices: %w", err)
	}

	return nil
}

