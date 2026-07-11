package topology

import (
	"context"
	"errors"
	"fmt"
	mrand "math/rand"
	"strings"
	"time"

	"horsync/internal/config"
	"horsync/internal/models"
	"horsync/pkg/utils"

	"github.com/google/uuid"
)

// ErrDatabaseNotConfigured is returned when the PostgreSQL pool is not initialized.
var ErrDatabaseNotConfigured = errors.New("database not configured")

// ErrDeviceNotFound is returned when a device ID does not exist.
var ErrDeviceNotFound = errors.New("device not found")

// ErrEnrollmentTokenInvalid is returned when an enrollment token is invalid or expired.
var ErrEnrollmentTokenInvalid = errors.New("invalid or expired enrollment token")

func (m *Mesh) ListDevices(ctx context.Context) ([]models.Node, error) {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return nil, ErrDatabaseNotConfigured
	}

	rows, err := db.Pool.Query(
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
		ORDER BY created_at DESC
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("list devices: %w", err)
	}
	defer rows.Close()

	devices := make([]models.Node, 0)
	for rows.Next() {
		var (
			node            models.Node
			loadPercent     int
			uptimeSeconds   int64
			lastSeenAt      *time.Time
			createdAt       time.Time
			approvedAt      *time.Time
			fingerprintHash string
		)

		if err := rows.Scan(
			&node.ID,
			&node.Name,
			&node.Type,
			&node.Location,
			&node.Status,
			&node.IP,
			&node.OwnerEmail,
			&node.SyncMode,
			&loadPercent,
			&uptimeSeconds,
			&lastSeenAt,
			&createdAt,
			&approvedAt,
			&fingerprintHash,
		); err != nil {
			return nil, fmt.Errorf("scan device: %w", err)
		}

		node.Load = fmt.Sprintf("%d%%", loadPercent)
		node.Uptime = utils.FormatUptime(uptimeSeconds)
		node.LastSeen = utils.FormatLastSeen(lastSeenAt, "Awaiting approval")
		node.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if approvedAt != nil {
			node.ApprovedAt = approvedAt.UTC().Format(time.RFC3339)
		}
		node.FingerprintPreview = utils.PreviewFingerprint(fingerprintHash)
		devices = append(devices, node)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate devices: %w", err)
	}

	return devices, nil
}

func (m *Mesh) RegisterDevice(ctx context.Context, input models.DeviceRegistrationInput) (models.Node, error) {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.Node{}, ErrDatabaseNotConfigured
	}

	enrollmentID, fallbackType, fallbackLocation, fallbackEmail, fallbackSyncMode, err := resolveEnrollment(ctx, db, input.EnrollmentToken)
	if err != nil {
		return models.Node{}, err
	}

	now := time.Now().UTC()
	device := models.Node{
		ID:         buildDeviceID(input.Name),
		Name:       strings.TrimSpace(input.Name),
		Type:       utils.NormalizeValue(input.Type, fallbackType),
		Location:   utils.NormalizeValue(input.Location, fallbackLocation),
		Status:     "pending",
		IP:         utils.NormalizeValue(input.IP, "PENDING_ASSIGNMENT"),
		Load:       "0%",
		Uptime:     "0D 00H",
		OwnerEmail: utils.NormalizeValue(input.OwnerEmail, fallbackEmail),
		SyncMode:   utils.NormalizeValue(input.SyncMode, fallbackSyncMode),
		LastSeen:   "Awaiting approval",
		CreatedAt:  now.Format(time.RFC3339),
	}

	deviceSecret, err := utils.GenerateRandomHex(32)
	if err != nil {
		return models.Node{}, fmt.Errorf("generate device secret: %w", err)
	}
	deviceSecretHash := utils.HashSHA256(deviceSecret)
	fingerprintHash := strings.ToLower(strings.TrimSpace(input.Fingerprint))

	_, err = db.Pool.Exec(
		ctx,
		`
		INSERT INTO devices (
			id,
			device_id,
			name,
			type,
			location,
			ip,
			owner_email,
			status,
			sync_mode,
			enrollment_id,
			device_secret_hash,
			fingerprint_hash,
			load_percent,
			uptime_seconds,
			last_seen_at,
			approved_at,
			created_at,
			updated_at
		)
		VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, 0, 0, NULL, NULL, $13, $13
		)
		`,
		uuid.NewString(),
		device.ID,
		device.Name,
		device.Type,
		device.Location,
		device.IP,
		device.OwnerEmail,
		device.Status,
		device.SyncMode,
		enrollmentID,
		deviceSecretHash,
		fingerprintHash,
		now,
	)
	if err != nil {
		return models.Node{}, fmt.Errorf("register device: %w", err)
	}

	if _, err := db.Pool.Exec(
		ctx,
		`
		UPDATE device_enrollments
		SET status = 'registered', registered_device_id = $2
		WHERE id = $1
		`,
		enrollmentID,
		device.ID,
	); err != nil {
		return models.Node{}, fmt.Errorf("update enrollment registration: %w", err)
	}

	device.DeviceSecret = deviceSecret
	_ = db.WriteAuditLog(ctx, "device.register", device.OwnerEmail, "device", device.ID, "success", device.Name)
	return device, nil
}

func (m *Mesh) ApproveDevice(ctx context.Context, deviceID string) (models.Node, error) {
	return m.updateDeviceStatus(ctx, deviceID, "active")
}

func (m *Mesh) RejectDevice(ctx context.Context, deviceID string) (models.Node, error) {
	return m.updateDeviceStatus(ctx, deviceID, "rejected")
}

func (m *Mesh) updateDeviceStatus(ctx context.Context, deviceID string, status string) (models.Node, error) {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.Node{}, ErrDatabaseNotConfigured
	}

	now := time.Now().UTC()
	loadPercent := 0
	uptimeSeconds := int64(0)
	var lastSeenAt *time.Time
	var approvedAt *time.Time

	if status == "active" {
		loadPercent = mrand.Intn(55) + 15
		uptimeSeconds = int64(mrand.Intn(72)+1) * 3600
		lastSeenAt = &now
		approvedAt = &now
	}

	commandTag, err := db.Pool.Exec(
		ctx,
		`
		UPDATE devices
		SET
			status = $2,
			load_percent = $3,
			uptime_seconds = $4,
			last_seen_at = $5,
			approved_at = $6,
			updated_at = $7
		WHERE device_id = $1
		`,
		deviceID,
		status,
		loadPercent,
		uptimeSeconds,
		lastSeenAt,
		approvedAt,
		now,
	)
	if err != nil {
		return models.Node{}, fmt.Errorf("update device status: %w", err)
	}

	if commandTag.RowsAffected() == 0 {
		return models.Node{}, ErrDeviceNotFound
	}

	device, err := m.getDeviceByID(ctx, deviceID)
	if err != nil {
		return models.Node{}, err
	}

	message := "device rejected"
	if status == "active" {
		message = "device approved"
	}
	_ = db.WriteAuditLog(ctx, "device."+status, "control-plane", "device", deviceID, "success", message)
	return device, nil
}

func (m *Mesh) getDeviceByID(ctx context.Context, deviceID string) (models.Node, error) {
	db := config.GetDatabase()
	if db == nil || db.Pool == nil {
		return models.Node{}, ErrDatabaseNotConfigured
	}

	var (
		node            models.Node
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
		`,
		deviceID,
	).Scan(
		&node.ID,
		&node.Name,
		&node.Type,
		&node.Location,
		&node.Status,
		&node.IP,
		&node.OwnerEmail,
		&node.SyncMode,
		&loadPercent,
		&uptimeSeconds,
		&lastSeenAt,
		&createdAt,
		&approvedAt,
		&fingerprintHash,
	)
	if err != nil {
		return models.Node{}, fmt.Errorf("get device: %w", err)
	}

	node.Load = fmt.Sprintf("%d%%", loadPercent)
	node.Uptime = utils.FormatUptime(uptimeSeconds)
	node.LastSeen = utils.FormatLastSeen(lastSeenAt, "Awaiting approval")
	node.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	if approvedAt != nil {
		node.ApprovedAt = approvedAt.UTC().Format(time.RFC3339)
	}
	node.FingerprintPreview = utils.PreviewFingerprint(fingerprintHash)
	return node, nil
}

func resolveEnrollment(ctx context.Context, db *config.Database, token string) (string, string, string, string, string, error) {
	var (
		id         string
		deviceType string
		location   string
		email      string
		syncMode   string
	)

	err := db.Pool.QueryRow(
		ctx,
		`
		SELECT id, device_type, location, owner_email, sync_mode
		FROM device_enrollments
		WHERE token_hash = $1
			AND status = 'pending_registration'
			AND expires_at > NOW()
		`,
		utils.HashSHA256(token),
	).Scan(&id, &deviceType, &location, &email, &syncMode)
	if err != nil {
		return "", "", "", "", "", ErrEnrollmentTokenInvalid
	}

	return id, deviceType, location, email, syncMode, nil
}

func buildDeviceID(name string) string {
	prefix := strings.ToUpper(strings.TrimSpace(name))
	prefix = strings.ReplaceAll(prefix, " ", "-")
	if prefix == "" {
		prefix = "DEVICE"
	}
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}

	return fmt.Sprintf("YVS-%s-%s", prefix, strings.ToUpper(uuid.NewString()[:6]))
}
