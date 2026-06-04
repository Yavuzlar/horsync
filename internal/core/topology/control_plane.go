package topology

import (
	"context"
	crand "crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	mrand "math/rand"
	"strings"
	"time"

	"horsync/internal/config"
	"horsync/internal/models"

	"github.com/google/uuid"
)

var ErrDatabaseNotConfigured = errors.New("database not configured")
var ErrDeviceNotFound = errors.New("device not found")
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
		node.Uptime = formatUptime(uptimeSeconds)
		node.LastSeen = formatLastSeen(lastSeenAt)
		node.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		if approvedAt != nil {
			node.ApprovedAt = approvedAt.UTC().Format(time.RFC3339)
		}
		node.FingerprintPreview = previewFingerprint(fingerprintHash)
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
		Type:       normalizeOrDefault(input.Type, fallbackType),
		Location:   normalizeOrDefault(input.Location, fallbackLocation),
		Status:     "pending",
		IP:         normalizeOrDefault(input.IP, "PENDING_ASSIGNMENT"),
		Load:       "0%",
		Uptime:     "0D 00H",
		OwnerEmail: normalizeOrDefault(input.OwnerEmail, fallbackEmail),
		SyncMode:   normalizeOrDefault(input.SyncMode, fallbackSyncMode),
		LastSeen:   "Awaiting approval",
		CreatedAt:  now.Format(time.RFC3339),
	}

	deviceSecretHash := hashOpaque(input.EnrollmentToken + ":" + device.ID)
	deviceSecret, err := generateDeviceSecret()
	if err != nil {
		return models.Node{}, fmt.Errorf("generate device secret: %w", err)
	}
	deviceSecretHash = hashOpaque(deviceSecret)
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
	node.Uptime = formatUptime(uptimeSeconds)
	node.LastSeen = formatLastSeen(lastSeenAt)
	node.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	if approvedAt != nil {
		node.ApprovedAt = approvedAt.UTC().Format(time.RFC3339)
	}
	node.FingerprintPreview = previewFingerprint(fingerprintHash)
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
		hashOpaque(token),
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

func normalizeOrDefault(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}

	return trimmed
}

func formatUptime(totalSeconds int64) string {
	if totalSeconds <= 0 {
		return "0D 00H"
	}

	duration := time.Duration(totalSeconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour

	return fmt.Sprintf("%dD %02dH", days, hours)
}

func formatLastSeen(value *time.Time) string {
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

func hashOpaque(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func previewFingerprint(value string) string {
	if len(value) <= 12 {
		return value
	}

	return value[:12]
}

func generateDeviceSecret() (string, error) {
	raw := make([]byte, 32)
	if _, err := crand.Read(raw); err != nil {
		return "", err
	}

	return hex.EncodeToString(raw), nil
}

