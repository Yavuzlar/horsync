package middleware

import (
	"strings"

	"horsync/internal/config"
	"horsync/internal/models"

	"github.com/gofiber/fiber/v2"
)

const userContextKey = "auth_user"

// RequireAuth validates the bearer token and sets the authenticated user in context.
func RequireAuth(c *fiber.Ctx) error {
	db := config.GetDatabase()
	if db == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error": "database not configured",
		})
	}

	header := strings.TrimSpace(c.Get("Authorization"))
	if !strings.HasPrefix(header, "Bearer ") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing bearer token",
		})
	}

	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "missing bearer token",
		})
	}

	user, err := db.ValidateSession(c.Context(), token)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid session",
		})
	}

	c.Locals(userContextKey, user)
	return c.Next()
}

// CurrentUser retrieves the authenticated user from the request context.
func CurrentUser(c *fiber.Ctx) models.User {
	if value := c.Locals(userContextKey); value != nil {
		if user, ok := value.(models.User); ok {
			return user
		}
	}

	return models.User{}
}

