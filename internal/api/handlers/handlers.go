package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"horsync/internal/api/middleware"
	"horsync/internal/config"
	"horsync/internal/core/engine"
	"horsync/internal/core/sysmonitor"
	"horsync/internal/core/topology"
	"horsync/internal/core/transfer"
	"horsync/internal/core/vault"
	"horsync/internal/core/p2p"
	"horsync/internal/models"

	"github.com/gofiber/fiber/v2"
)

// Login authenticates a user and returns a session token.
func Login(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	var input models.LoginInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid login payload",
		})
	}

	session, err := db.CreateSession(c.Context(), input)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(session)
}

// Me returns the currently authenticated user.
func Me(c *fiber.Ctx) error {
	return c.JSON(middleware.CurrentUser(c))
}

// GetStats returns the current system monitoring stats.
func GetStats(c *fiber.Ctx) error {
	monitor := sysmonitor.GetInstance()
	return c.JSON(monitor.GetStats())
}

// GetPerformance returns the system performance history.
func GetPerformance(c *fiber.Ctx) error {
	monitor := sysmonitor.GetInstance()
	return c.JSON(monitor.GetPerformanceHistory())
}

// GetNodes returns all nodes in the P2P topology mesh.
func GetNodes(c *fiber.Ctx) error {
	mesh := topology.GetInstance()
	return c.JSON(mesh.GetNodes())
}

// GetRules returns all engine rules.
func GetRules(c *fiber.Ctx) error {
	eng := engine.GetInstance()
	return c.JSON(eng.GetRules())
}

// ToggleRule enables or disables a rule by its ID.
func ToggleRule(c *fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid rule id",
		})
	}

	eng := engine.GetInstance()
	rule, err := eng.ToggleRule(id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	db := config.GetDatabase()
	if db != nil {
		user := middleware.CurrentUser(c)
		statusStr := "disabled"
		if rule.Active {
			statusStr = "enabled"
		}
		_ = db.WriteAuditLog(c.Context(), "rule.toggle", user.Email, "rule", strconv.Itoa(rule.ID), "success", fmt.Sprintf("rule %s %s", rule.Name, statusStr))
	}

	return c.JSON(rule)
}

// GetFiles returns the list of managed files.
func GetFiles(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db != nil {
		if files, err := db.ListFiles(c.Context()); err == nil {
			return c.JSON(files)
		}
	}

	eng := engine.GetInstance()
	return c.JSON(eng.GetFiles())
}

// GetSecurityLogs returns recent security audit logs.
func GetSecurityLogs(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db != nil {
		if logs, err := db.GetAuditLogs(c.Context(), 50); err == nil {
			return c.JSON(logs)
		}
	}

	v := vault.GetInstance()
	return c.JSON(v.GetLogs())
}

// ListAuditLogs returns a list of audit log entries.
func ListAuditLogs(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	logs, err := db.GetAuditLogs(c.Context(), 100)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(logs)
}

// ListDevices returns all registered devices.
func ListDevices(c *fiber.Ctx) error {
	mesh := topology.GetInstance()
	devices, err := mesh.ListDevices(c.Context())
	if err != nil {
		return handleControlPlaneError(c, err)
	}

	return c.JSON(devices)
}

// RegisterDevice registers a new device in the topology mesh.
func RegisterDevice(c *fiber.Ctx) error {
	var input models.DeviceRegistrationInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid device payload",
		})
	}

	if strings.TrimSpace(input.EnrollmentToken) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "enrollment token is required",
		})
	}
	if strings.TrimSpace(input.Fingerprint) == "" {
		input.Fingerprint = middleware.BuildDeviceFingerprint(
			c,
			input.Name,
			input.Type,
			input.Location,
			input.OwnerEmail,
			input.SyncMode,
		)
	}

	mesh := topology.GetInstance()
	device, err := mesh.RegisterDevice(c.Context(), input)
	if err != nil {
		return handleControlPlaneError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(device)
}

// CreateUploadSession creates a new chunked upload session.
func CreateUploadSession(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	var input models.UploadSessionInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid upload session payload",
		})
	}

	user := middleware.CurrentUser(c)
	fingerprint := strings.TrimSpace(c.Get("X-Device-Fingerprint"))
	if fingerprint == "" {
		fingerprint = middleware.BuildDeviceFingerprint(c, input.FileName, input.ContentType, input.SourceDeviceID)
	}

	session, err := transfer.GetInstance().CreateUploadSession(c.Context(), user, input, fingerprint)
	if err != nil {
		return handleUploadError(c, err)
	}

	_ = db.WriteAuditLog(c.Context(), "upload.session.create", user.Email, "upload-session", session.ID, "success", session.FileName)
	return c.Status(fiber.StatusCreated).JSON(session)
}

// UploadChunk stores a single chunk for an upload session.
func UploadChunk(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	chunkIndex, err := strconv.Atoi(c.Params("index"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid chunk index",
		})
	}

	result, err := transfer.GetInstance().SaveChunk(
		c.Context(),
		c.Params("id"),
		chunkIndex,
		c.BodyRaw(),
		c.Get("X-Chunk-SHA256"),
	)
	if err != nil {
		return handleUploadError(c, err)
	}

	user := middleware.CurrentUser(c)
	_ = db.WriteAuditLog(c.Context(), "upload.chunk.accepted", user.Email, "upload-session", result.SessionID, "success", "chunk stored")
	return c.JSON(result)
}

// FinalizeUpload finalizes a completed upload session and triggers replication.
func FinalizeUpload(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	fileEntry, err := transfer.GetInstance().FinalizeUpload(c.Context(), c.Params("id"))
	if err != nil {
		return handleUploadError(c, err)
	}

	user := middleware.CurrentUser(c)
	status := "success"
	message := "upload finalized"
	if fileEntry.Integrity == "mismatch" {
		status = "warning"
		message = "upload finalized with sha256 mismatch"
	}
	_ = db.WriteAuditLog(c.Context(), "upload.finalize", user.Email, "upload-session", c.Params("id"), status, message)
	if fileEntry.Integrity != "mismatch" {
		if session, sessionErr := transfer.GetInstance().GetUploadSession(c.Context(), c.Params("id")); sessionErr == nil {
			if jobs, jobErr := db.CreateReplicationJobs(c.Context(), session.ID, session.SourceDeviceID); jobErr == nil && len(jobs) > 0 {
				_ = db.WriteAuditLog(c.Context(), "replication.queue", user.Email, "upload-session", session.ID, "success", "replication jobs created")
			}
		}
	}

	return c.JSON(fileEntry)
}

// GetUploadSession returns the details of an upload session.
func GetUploadSession(c *fiber.Ctx) error {
	session, err := transfer.GetInstance().GetUploadSession(c.Context(), c.Params("id"))
	if err != nil {
		return handleUploadError(c, err)
	}

	return c.JSON(session)
}

// ListAgentJobs returns replication jobs for the authenticated device agent.
func ListAgentJobs(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	device := middleware.CurrentDevice(c)
	jobs, err := db.ListReplicationJobsForDevice(c.Context(), device.ID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(jobs)
}

// GetAgentManifest returns the replication manifest for a specific job.
func GetAgentManifest(c *fiber.Ctx) error {
	device := middleware.CurrentDevice(c)
	manifest, err := transfer.GetInstance().GetReplicationManifest(c.Context(), c.Params("id"), device.ID)
	if err != nil {
		return handleUploadError(c, err)
	}

	return c.JSON(manifest)
}

// DownloadAgentChunk streams a single replication chunk to the device agent.
func DownloadAgentChunk(c *fiber.Ctx) error {
	device := middleware.CurrentDevice(c)
	chunkIndex, err := strconv.Atoi(c.Params("index"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid chunk index",
		})
	}

	payload, meta, err := transfer.GetInstance().ReadReplicationChunk(c.Context(), c.Params("id"), device.ID, chunkIndex)
	if err != nil {
		return handleUploadError(c, err)
	}

	c.Set("Content-Type", "application/octet-stream")
	c.Set("X-Chunk-Index", strconv.Itoa(meta.ChunkIndex))
	c.Set("X-Chunk-SHA256", meta.ChunkSHA256)
	return c.Send(payload)
}

// CompleteAgentJob marks a replication job as completed by the device agent.
func CompleteAgentJob(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	var input models.ReplicationAckInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid replication ack payload",
		})
	}

	device := middleware.CurrentDevice(c)
	job, err := transfer.GetInstance().CompleteReplication(c.Context(), c.Params("id"), device.ID, input)
	if err != nil {
		return handleUploadError(c, err)
	}

	message := "replication committed"
	if strings.TrimSpace(job.LastError) != "" {
		message = job.LastError
	}
	_ = db.WriteAuditLog(c.Context(), "replication.complete", device.ID, "replication-job", job.ID, job.Status, message)
	return c.JSON(job)
}

// ApproveDevice approves a pending device registration.
func ApproveDevice(c *fiber.Ctx) error {
	mesh := topology.GetInstance()
	device, err := mesh.ApproveDevice(c.Context(), c.Params("id"))
	if err != nil {
		return handleControlPlaneError(c, err)
	}

	return c.JSON(device)
}

// RejectDevice rejects a pending device registration.
func RejectDevice(c *fiber.Ctx) error {
	mesh := topology.GetInstance()
	device, err := mesh.RejectDevice(c.Context(), c.Params("id"))
	if err != nil {
		return handleControlPlaneError(c, err)
	}

	return c.JSON(device)
}

// ListDeviceEnrollments returns all device enrollment tokens.
func ListDeviceEnrollments(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	enrollments, err := db.ListDeviceEnrollments(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(enrollments)
}

// CreateDeviceEnrollment creates a new device enrollment token.
func CreateDeviceEnrollment(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	var input models.DeviceEnrollmentInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid enrollment payload",
		})
	}

	user := middleware.CurrentUser(c)
	enrollment, err := db.CreateDeviceEnrollment(c.Context(), user, input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.Status(fiber.StatusCreated).JSON(enrollment)
}

// GetInstanceSettings returns the current instance configuration.
func GetInstanceSettings(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	settings, err := db.GetInstanceSettings(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return c.JSON(settings)
}

// UpdateInstanceSettings updates the instance configuration.
func UpdateInstanceSettings(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	var input models.InstanceSettings
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid settings payload",
		})
	}

	settings, err := db.UpdateInstanceSettings(c.Context(), input)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	user := middleware.CurrentUser(c)
	_ = db.WriteAuditLog(c.Context(), "settings.update", user.Email, "instance", "primary", "success", "instance settings updated")

	return c.JSON(settings)
}

// handleControlPlaneError maps control-plane errors to HTTP responses.
func handleControlPlaneError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, fiber.ErrBadRequest):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, topology.ErrDatabaseNotConfigured):
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, topology.ErrDeviceNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, topology.ErrEnrollmentTokenInvalid):
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
}

// handleUploadError maps upload-related errors to HTTP responses.
func handleUploadError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, transfer.ErrInvalidUploadSession):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrChunkTooLarge):
		return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrInvalidChunkIndex):
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrChunkIntegrityMismatch):
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrUploadAlreadyCompleted):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrUploadIncomplete):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrUploadSessionNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrUploadStorageUnavailable):
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrReplicationJobNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, transfer.ErrReplicationChunkNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, config.ErrChunkAlreadyUploaded):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
}

// GetP2PPeers returns the active and discovered P2P peers.
func GetP2PPeers(c *fiber.Ctx) error {
	engine := p2p.GetInstance()
	return c.JSON(fiber.Map{
		"active":     engine.GetActivePeers(),
		"discovered": engine.GetDiscoveredPeers(),
	})
}

// DownloadHorsyncExecutable serves the Horsync agent executable for download.
func DownloadHorsyncExecutable(c *fiber.Ctx) error {
	path := "bin/horsync.exe"
	if _, err := os.Stat(path); err != nil {
		execPath, err := os.Executable()
		if err == nil {
			path = filepath.Join(filepath.Dir(execPath), "bin", "horsync.exe")
			if _, err := os.Stat(path); err != nil {
				path = filepath.Join(filepath.Dir(execPath), "horsync.exe")
			}
		}
	}

	if _, err := os.Stat(path); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "horsync executable file not found on server",
		})
	}

	c.Set("Content-Disposition", "attachment; filename=horsync.exe")
	return c.SendFile(path)
}

// WipeFileMetadata removes EXIF or document metadata from a completed upload.
func WipeFileMetadata(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	fileName := c.Params("name")
	var (
		sessionID    string
		storagePath  string
		actualSHA256 string
	)

	// Fetch from database
	err := db.Pool.QueryRow(
		c.Context(),
		`SELECT id, storage_path, actual_sha256 FROM file_upload_sessions WHERE file_name = $1 AND status = 'completed' LIMIT 1`,
		fileName,
	).Scan(&sessionID, &storagePath, &actualSHA256)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "file session not found"})
	}

	// Resolve the on-disk path through the transfer manager so the configured
	// upload storage directory is honoured rather than a hard-coded "data/uploads".
	filePath, err := transfer.GetInstance().ResolveUploadFile(c.Context(), sessionID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "file session not found"})
	}
	if _, err := os.Stat(filePath); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "file not found on disk"})
	}

	ext := strings.ToLower(filepath.Ext(fileName))
	wiped := false
	eng := engine.GetInstance()
	if ext == ".jpg" || ext == ".jpeg" || ext == ".png" {
		if err := transfer.GetInstance().WipeEXIF(filePath); err == nil {
			wiped = true
			eng.TriggerRule("WIPE_EXIF_METADATA")
		}
	} else if ext == ".pdf" || ext == ".docx" || ext == ".xlsx" {
		if err := transfer.GetInstance().WipeDocMetadata(filePath); err == nil {
			wiped = true
			eng.TriggerRule("WIPE_DOCUMENT_METADATA")
		}
	}

	if !wiped {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "no metadata wiping rule applies to this file type"})
	}

	// Recalculate hash and update session in database
	file, err := os.Open(filePath)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	defer file.Close()

	hasher := sha256.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	newSHA256 := hex.EncodeToString(hasher.Sum(nil))

	// Update session in database
	_, err = db.Pool.Exec(
		c.Context(),
		`
		UPDATE file_upload_sessions
		SET
			actual_sha256 = $2,
			integrity_status = 'verified',
			updated_at = NOW()
		WHERE id = $1
		`,
		sessionID,
		newSHA256,
	)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	user := middleware.CurrentUser(c)
	_ = db.WriteAuditLog(c.Context(), "file.metadata.wipe", user.Email, "file", sessionID, "success", "metadata wiped manually via dashboard")

	return c.JSON(fiber.Map{
		"status": "success",
		"sha256": newSHA256,
	})
}


