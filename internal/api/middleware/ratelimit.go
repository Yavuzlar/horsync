package middleware

import (
	"fmt"
	"sync"
	"time"

	"horsync/internal/config"

	"github.com/gofiber/fiber/v2"
)

type rateLimitEntry struct {
	count      int
	windowEnds time.Time
}

type rateLimiter struct {
	mu      sync.Mutex
	entries map[string]rateLimitEntry
}

var sharedRateLimiter = &rateLimiter{
	entries: make(map[string]rateLimitEntry),
}

func FixedWindowRateLimit(name string, limit int, window time.Duration, keyFunc func(*fiber.Ctx) string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if limit <= 0 || window <= 0 {
			return c.Next()
		}

		key := c.IP()
		if keyFunc != nil {
			if resolved := keyFunc(c); resolved != "" {
				key = resolved
			}
		}

		compositeKey := fmt.Sprintf("%s:%s", name, key)
		remaining := sharedRateLimiter.allow(compositeKey, limit, window)
		if remaining < 0 {
			if db := config.GetDatabase(); db != nil {
				_ = db.WriteAuditLog(c.Context(), "rate_limit.block", key, "rate-limit", name, "blocked", "rate limit exceeded")
			}
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "rate limit exceeded",
			})
		}

		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", limit))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		return c.Next()
	}
}

func (r *rateLimiter) allow(key string, limit int, window time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now().UTC()
	entry, ok := r.entries[key]
	if !ok || now.After(entry.windowEnds) {
		r.entries[key] = rateLimitEntry{
			count:      1,
			windowEnds: now.Add(window),
		}
		return limit - 1
	}

	entry.count++
	r.entries[key] = entry
	if entry.count > limit {
		return -1
	}

	return limit - entry.count
}

