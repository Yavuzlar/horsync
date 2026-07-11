package middleware

import (
	"strings"

	"horsync/internal/config"
	"horsync/internal/models"

	"github.com/gofiber/fiber/v2"
)

const deviceContextKey = "auth_device"

// RequireDeviceAgent validates device credentials and sets the authenticated device in context.
func RequireDeviceAgent(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	auth := models.DeviceAgentAuth{
		DeviceID:     strings.TrimSpace(c.Get("X-Device-ID")),
		DeviceSecret: strings.TrimSpace(c.Get("X-Device-Secret")),
	}
	if auth.DeviceID == "" || auth.DeviceSecret == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing device credentials",
		})
	}

	device, err := db.ValidateDeviceAgent(c.Context(), auth)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid device credentials",
		})
	}

	c.Locals(deviceContextKey, device)
	return c.Next()
}

// CurrentDevice retrieves the authenticated device from the request context.
func CurrentDevice(c *fiber.Ctx) models.Node {
	if value := c.Locals(deviceContextKey); value != nil {
		if device, ok := value.(models.Node); ok {
			return device
		}
	}

	return models.Node{}
}

