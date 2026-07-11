package middleware

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

func TestFixedWindowRateLimitDisabled(t *testing.T) {
	app := fiber.New()
	app.Get("/test", FixedWindowRateLimit("test", 0, 0, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 200 {
		t.Errorf("Expected 200 when rate limit disabled, got %d", resp.StatusCode)
	}
}

func TestFixedWindowRateLimitBasic(t *testing.T) {
	app := fiber.New()
	app.Get("/test", FixedWindowRateLimit("basic-test", 3, 10*time.Second, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// First 3 requests should succeed
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest("GET", "/test", nil)
		resp, _ := app.Test(req, 1000)
		if resp.StatusCode != 200 {
			t.Errorf("Request %d: expected 200, got %d", i+1, resp.StatusCode)
		}
	}

	// 4th request should be rate limited
	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, 1000)
	if resp.StatusCode != 429 {
		t.Errorf("Expected 429 rate limited, got %d", resp.StatusCode)
	}
}

func TestFixedWindowRateLimitCustomKey(t *testing.T) {
	customKeyCalls := 0
	app := fiber.New()
	app.Get("/test", FixedWindowRateLimit("keyed", 1, 10*time.Second, func(c *fiber.Ctx) string {
		customKeyCalls++
		return "custom-" + c.Get("X-User-ID")
	}), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Different users should have separate counters
	req1 := httptest.NewRequest("GET", "/test", nil)
	req1.Header.Set("X-User-ID", "alice")
	resp1, _ := app.Test(req1, 1000)
	if resp1.StatusCode != 200 {
		t.Errorf("Alice request 1: expected 200, got %d", resp1.StatusCode)
	}

	// Alice exceeded
	req2 := httptest.NewRequest("GET", "/test", nil)
	req2.Header.Set("X-User-ID", "alice")
	resp2, _ := app.Test(req2, 1000)
	if resp2.StatusCode != 429 {
		t.Errorf("Alice request 2: expected 429, got %d", resp2.StatusCode)
	}

	// Bob should still succeed (different key)
	req3 := httptest.NewRequest("GET", "/test", nil)
	req3.Header.Set("X-User-ID", "bob")
	resp3, _ := app.Test(req3, 1000)
	if resp3.StatusCode != 200 {
		t.Errorf("Bob request 1: expected 200, got %d", resp3.StatusCode)
	}

	if customKeyCalls == 0 {
		t.Error("Custom key function was never called")
	}
}

func TestFixedWindowRateLimitWindowReset(t *testing.T) {
	app := fiber.New()
	// Very short window (50ms) for testing
	app.Get("/test", FixedWindowRateLimit("reset-test", 1, 50*time.Millisecond, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// First request succeeds
	req1 := httptest.NewRequest("GET", "/test", nil)
	resp1, _ := app.Test(req1, 1000)
	if resp1.StatusCode != 200 {
		t.Errorf("First request: expected 200, got %d", resp1.StatusCode)
	}

	// Second request blocked
	req2 := httptest.NewRequest("GET", "/test", nil)
	resp2, _ := app.Test(req2, 1000)
	if resp2.StatusCode != 429 {
		t.Errorf("Second request: expected 429, got %d", resp2.StatusCode)
	}

	// Wait for window to reset
	time.Sleep(60 * time.Millisecond)

	// Should succeed again
	req3 := httptest.NewRequest("GET", "/test", nil)
	resp3, _ := app.Test(req3, 1000)
	if resp3.StatusCode != 200 {
		t.Errorf("After window reset: expected 200, got %d", resp3.StatusCode)
	}
}

func TestFixedWindowRateLimitRateLimitHeaders(t *testing.T) {
	app := fiber.New()
	app.Get("/test", FixedWindowRateLimit("header-test", 5, 10*time.Second, nil), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/test", nil)
	resp, _ := app.Test(req, 1000)

	if resp.Header.Get("X-RateLimit-Limit") != "5" {
		t.Errorf("Expected X-RateLimit-Limit: 5, got %s", resp.Header.Get("X-RateLimit-Limit"))
	}
	if resp.Header.Get("X-RateLimit-Remaining") != "4" {
		t.Errorf("Expected X-RateLimit-Remaining: 4, got %s", resp.Header.Get("X-RateLimit-Remaining"))
	}
}
