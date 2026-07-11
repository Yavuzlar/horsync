package config

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"horsync/internal/models"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const sessionTTL = 24 * time.Hour

// CreateSession authenticates a user by email and password, creates an auth session, and returns the raw session token.
func (db *Database) CreateSession(ctx context.Context, input models.LoginInput) (models.AuthSession, error) {
	user, passwordHash, err := db.getUserByEmail(ctx, input.Email)
	if err != nil {
		return models.AuthSession{}, err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(input.Password)); err != nil {
		_ = db.WriteAuditLog(ctx, "auth.login", input.Email, "user", user.ID, "failed", "invalid credentials")
		return models.AuthSession{}, fmt.Errorf("invalid credentials")
	}

	rawToken, tokenHash, err := generateOpaqueToken()
	if err != nil {
		return models.AuthSession{}, err
	}

	expiresAt := time.Now().UTC().Add(sessionTTL)
	if _, err := db.Pool.Exec(
		ctx,
		`
		INSERT INTO auth_sessions (
			id,
			user_id,
			token_hash,
			expires_at,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5)
		`,
		uuid.NewString(),
		user.ID,
		tokenHash,
		expiresAt,
		time.Now().UTC(),
	); err != nil {
		return models.AuthSession{}, fmt.Errorf("create auth session: %w", err)
	}

	_ = db.WriteAuditLog(ctx, "auth.login", user.Email, "user", user.ID, "success", "admin session started")

	return models.AuthSession{
		Token:     rawToken,
		User:      user,
		ExpiresAt: expiresAt.Format(time.RFC3339),
	}, nil
}

// ValidateSession looks up an active (non-expired) auth session by its token hash and returns the associated user.
func (db *Database) ValidateSession(ctx context.Context, token string) (models.User, error) {
	var (
		user      models.User
		createdAt time.Time
	)

	row := db.Pool.QueryRow(
		ctx,
		`
		SELECT
			u.id,
			u.email,
			u.name,
			u.role,
			u.created_at
		FROM auth_sessions s
		INNER JOIN users u ON u.id = s.user_id
		WHERE s.token_hash = $1 AND s.expires_at > NOW()
		`,
		hashToken(token),
	)
	if err := row.Scan(&user.ID, &user.Email, &user.Name, &user.Role, &createdAt); err != nil {
		return models.User{}, fmt.Errorf("invalid session")
	}

	user.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return user, nil
}

// CreateDeviceEnrollment generates an opaque enrollment token and registers a new device enrollment record.
func (db *Database) CreateDeviceEnrollment(ctx context.Context, actor models.User, input models.DeviceEnrollmentInput) (models.DeviceEnrollment, error) {
	rawToken, tokenHash, err := generateOpaqueToken()
	if err != nil {
		return models.DeviceEnrollment{}, err
	}

	now := time.Now().UTC()
	expiresIn := input.ExpiresIn
	if expiresIn <= 0 {
		expiresIn = 24
	}
	expiresAt := now.Add(time.Duration(expiresIn) * time.Hour)

	enrollment := models.DeviceEnrollment{
		ID:           uuid.NewString(),
		Token:        rawToken,
		TokenPreview: previewToken(rawToken),
		Label:        strings.TrimSpace(input.Label),
		DeviceType:   normalizeValue(input.DeviceType, "DESKTOP"),
		Location:     normalizeValue(input.Location, "UNASSIGNED"),
		OwnerEmail:   normalizeValue(input.OwnerEmail, actor.Email),
		SyncMode:     normalizeValue(input.SyncMode, "bidirectional"),
		ExpiresAt:    expiresAt.Format(time.RFC3339),
		CreatedAt:    now.Format(time.RFC3339),
		CreatedBy:    actor.Email,
		Status:       "pending_registration",
	}

	if enrollment.Label == "" {
		enrollment.Label = fmt.Sprintf("%s enrollment", actor.Name)
	}

	_, err = db.Pool.Exec(
		ctx,
		`
		INSERT INTO device_enrollments (
			id,
			token_hash,
			token_preview,
			label,
			device_type,
			location,
			owner_email,
			sync_mode,
			created_by,
			status,
			expires_at,
			created_at,
			registered_device_id
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, '')
		`,
		enrollment.ID,
		tokenHash,
		enrollment.TokenPreview,
		enrollment.Label,
		enrollment.DeviceType,
		enrollment.Location,
		enrollment.OwnerEmail,
		enrollment.SyncMode,
		actor.ID,
		enrollment.Status,
		expiresAt,
		now,
	)
	if err != nil {
		return models.DeviceEnrollment{}, fmt.Errorf("create device enrollment: %w", err)
	}

	_ = db.WriteAuditLog(ctx, "device.enrollment.create", actor.Email, "enrollment", enrollment.ID, "success", enrollment.Label)
	return enrollment, nil
}

// ListDeviceEnrollments returns all device enrollment records ordered by creation date descending.
func (db *Database) ListDeviceEnrollments(ctx context.Context) ([]models.DeviceEnrollment, error) {
	rows, err := db.Pool.Query(
		ctx,
		`
		SELECT
			e.id,
			e.token_preview,
			e.label,
			e.device_type,
			e.location,
			e.owner_email,
			e.sync_mode,
			e.expires_at,
			e.created_at,
			u.email,
			e.status,
			e.registered_device_id
		FROM device_enrollments e
		INNER JOIN users u ON u.id = e.created_by
		ORDER BY e.created_at DESC
		`,
	)
	if err != nil {
		return nil, fmt.Errorf("list device enrollments: %w", err)
	}
	defer rows.Close()

	result := make([]models.DeviceEnrollment, 0)
	for rows.Next() {
		var (
			item      models.DeviceEnrollment
			expiresAt time.Time
			createdAt time.Time
		)

		if err := rows.Scan(
			&item.ID,
			&item.TokenPreview,
			&item.Label,
			&item.DeviceType,
			&item.Location,
			&item.OwnerEmail,
			&item.SyncMode,
			&expiresAt,
			&createdAt,
			&item.CreatedBy,
			&item.Status,
			&item.RegisteredDevice,
		); err != nil {
			return nil, fmt.Errorf("scan device enrollment: %w", err)
		}

		item.ExpiresAt = expiresAt.UTC().Format(time.RFC3339)
		item.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		result = append(result, item)
	}

	return result, rows.Err()
}

// GetAuditLogs retrieves the most recent audit log entries up to the given limit.
func (db *Database) GetAuditLogs(ctx context.Context, limit int) ([]models.AuditLog, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := db.Pool.Query(
		ctx,
		`
		SELECT id, action, actor, target_type, target_id, status, message, created_at
		FROM audit_logs
		ORDER BY created_at DESC
		LIMIT $1
		`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list audit logs: %w", err)
	}
	defer rows.Close()

	result := make([]models.AuditLog, 0, limit)
	for rows.Next() {
		var (
			entry     models.AuditLog
			createdAt time.Time
		)
		if err := rows.Scan(&entry.ID, &entry.Action, &entry.Actor, &entry.TargetType, &entry.TargetID, &entry.Status, &entry.Message, &createdAt); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}
		entry.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		result = append(result, entry)
	}

	return result, rows.Err()
}

// WriteAuditLog inserts a new audit trail entry recording the specified action and its outcome.
func (db *Database) WriteAuditLog(ctx context.Context, action string, actor string, targetType string, targetID string, status string, message string) error {
	_, err := db.Pool.Exec(
		ctx,
		`
		INSERT INTO audit_logs (
			id,
			action,
			actor,
			target_type,
			target_id,
			status,
			message,
			created_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
		uuid.NewString(),
		action,
		actor,
		targetType,
		targetID,
		status,
		message,
		time.Now().UTC(),
	)
	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}

	return nil
}

func (db *Database) getUserByEmail(ctx context.Context, email string) (models.User, string, error) {
	var (
		user         models.User
		passwordHash string
		createdAt    time.Time
	)

	err := db.Pool.QueryRow(
		ctx,
		`
		SELECT id, email, password_hash, name, role, created_at
		FROM users
		WHERE LOWER(email) = LOWER($1)
		`,
		strings.TrimSpace(email),
	).Scan(&user.ID, &user.Email, &passwordHash, &user.Name, &user.Role, &createdAt)
	if err != nil {
		return models.User{}, "", fmt.Errorf("invalid credentials")
	}

	user.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return user, passwordHash, nil
}

func generateOpaqueToken() (string, string, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}

	token := hex.EncodeToString(raw)
	return token, hashToken(token), nil
}

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func previewToken(token string) string {
	if len(token) < 12 {
		return token
	}

	return fmt.Sprintf("%s...%s", token[:6], token[len(token)-4:])
}

func normalizeValue(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}

	return trimmed
}

