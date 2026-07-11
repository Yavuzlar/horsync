package routes

import (
	"horsync/internal/api/handlers"
	"horsync/internal/api/middleware"
	"horsync/pkg/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

func Register(app *fiber.App) {
	api := app.Group("/api")

	api.Post("/auth/login", middleware.FixedWindowRateLimit("auth_login", 5, 5*time.Minute, nil), handlers.Login)
	api.Post("/devices/register", middleware.FixedWindowRateLimit("device_register", 10, 10*time.Minute, nil), handlers.RegisterDevice)
	api.Get("/downloads/horsync.exe", handlers.DownloadHorsyncExecutable)
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":   "ok",
			"version":  "1.2.4-beta",
			"local_ip": utils.GetLocalIP(),
		})
	})

	agent := api.Group("/agent", middleware.RequireDeviceAgent)
	agent.Get("/jobs", middleware.FixedWindowRateLimit("agent_jobs", 120, 10*time.Minute, func(c *fiber.Ctx) string {
		return c.Get("X-Device-ID")
	}), handlers.ListAgentJobs)
	agent.Get("/jobs/:id/manifest", handlers.GetAgentManifest)
	agent.Get("/jobs/:id/chunks/:index", handlers.DownloadAgentChunk)
	agent.Post("/jobs/:id/complete", handlers.CompleteAgentJob)

	protected := api.Group("", middleware.RequireAuth)
	protected.Get("/auth/me", handlers.Me)
	protected.Get("/stats", handlers.GetStats)
	protected.Get("/performance", handlers.GetPerformance)
	protected.Get("/nodes", handlers.GetNodes)
	protected.Get("/p2p/peers", handlers.GetP2PPeers)
	protected.Get("/rules", handlers.GetRules)
	protected.Post("/rules/:id/toggle", handlers.ToggleRule)
	protected.Get("/files", handlers.GetFiles)
	protected.Post("/files/:name/wipe-metadata", handlers.WipeFileMetadata)
	protected.Get("/security/logs", handlers.GetSecurityLogs)
	protected.Get("/audit/logs", handlers.ListAuditLogs)
	protected.Get("/devices", handlers.ListDevices)
	protected.Get("/device-enrollments", handlers.ListDeviceEnrollments)
	protected.Post("/device-enrollments", handlers.CreateDeviceEnrollment)
	protected.Post("/devices/:id/approve", handlers.ApproveDevice)
	protected.Post("/devices/:id/reject", handlers.RejectDevice)
	protected.Get("/settings/instance", handlers.GetInstanceSettings)
	protected.Put("/settings/instance", handlers.UpdateInstanceSettings)

	// Vault (Zero-Knowledge Encryption)
	protected.Get("/vault/status", handlers.VaultStatus)
	protected.Post("/vault/unlock", handlers.VaultUnlock)
	protected.Post("/vault/lock", handlers.VaultLock)
	protected.Post("/vault/encrypt", handlers.VaultEncrypt)
	protected.Post("/vault/decrypt", handlers.VaultDecrypt)

	protected.Post("/uploads/sessions", middleware.FixedWindowRateLimit("upload_session", 20, 10*time.Minute, nil), handlers.CreateUploadSession)
	protected.Get("/uploads/:id", handlers.GetUploadSession)
	protected.Put("/uploads/:id/chunks/:index", middleware.FixedWindowRateLimit("upload_chunk", 240, 10*time.Minute, func(c *fiber.Ctx) string {
		return c.Get("Authorization") + ":" + c.Params("id")
	}), handlers.UploadChunk)
	protected.Post("/uploads/:id/finalize", middleware.FixedWindowRateLimit("upload_finalize", 30, 10*time.Minute, nil), handlers.FinalizeUpload)
}


