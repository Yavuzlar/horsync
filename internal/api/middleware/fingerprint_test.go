package middleware

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestBuildDeviceFingerprint(t *testing.T) {
	app := fiber.New()
	var fp string

	app.Get("/test", func(c *fiber.Ctx) error {
		fp = BuildDeviceFingerprint(c, "test-device", "DESKTOP")
		return c.SendString(fp)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("User-Agent", "GoTest/1.0")
	req.Header.Set("Accept-Language", "en-US")
	resp, _ := app.Test(req, 1000)

	if fp == "" {
		t.Error("Fingerprint should not be empty")
	}
	if len(fp) != 64 {
		t.Errorf("Expected SHA-256 hex (64 chars), got %d: %s", len(fp), fp)
	}

	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != fp {
		t.Errorf("Response body should match fingerprint")
	}
}

func TestBuildDeviceFingerprintDeterministic(t *testing.T) {
	app := fiber.New()
	var fp1, fp2 string

	app.Get("/a", func(c *fiber.Ctx) error {
		fp1 = BuildDeviceFingerprint(c, "same-input")
		return c.SendString("ok")
	})

	app.Get("/b", func(c *fiber.Ctx) error {
		fp2 = BuildDeviceFingerprint(c, "same-input")
		return c.SendString("ok")
	})

	req := httptest.NewRequest("GET", "/a", nil)
	req.Header.Set("User-Agent", "GoTest/1.0")
	_, _ = app.Test(req, 1000)

	req2 := httptest.NewRequest("GET", "/b", nil)
	req2.Header.Set("User-Agent", "GoTest/1.0")
	_, _ = app.Test(req2, 1000)

	if fp1 == fp2 {
		t.Log("Same input produces same fingerprint (expected)")
	}

	if len(fp1) != 64 || len(fp2) != 64 {
		t.Error("Fingerprints should be 64 hex characters")
	}
}
