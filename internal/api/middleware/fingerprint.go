package middleware

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/gofiber/fiber/v2"
)

// BuildDeviceFingerprint generates a device fingerprint hash from request metadata and optional segments.
func BuildDeviceFingerprint(c *fiber.Ctx, segments ...string) string {
	parts := []string{
		strings.TrimSpace(c.IP()),
		strings.TrimSpace(c.Get("User-Agent")),
		strings.TrimSpace(c.Get("Accept-Language")),
		strings.TrimSpace(c.Get("X-Forwarded-For")),
		strings.TrimSpace(c.Get("X-Device-ID")),
	}
	parts = append(parts, segments...)

	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}
