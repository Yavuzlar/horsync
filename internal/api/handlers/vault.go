package handlers

import (
	"horsync/internal/config"
	"horsync/internal/core/vault"

	"github.com/gofiber/fiber/v2"
)

type VaultUnlockInput struct {
	Passphrase string `json:"passphrase"`
}

type VaultStatusResponse struct {
	IsUnlocked     bool   `json:"isUnlocked"`
	KeyFingerprint string `json:"keyFingerprint"`
}

// VaultUnlock unlocks the vault with a passphrase.
func VaultUnlock(c *fiber.Ctx) error {
	var input VaultUnlockInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "invalid payload, passphrase required",
		})
	}

	if input.Passphrase == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "passphrase is required",
		})
	}

	v := vault.GetInstance()
	if err := v.Unlock(input.Passphrase); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if db := config.GetDatabase(); db != nil {
		_ = db.WriteAuditLog(c.Context(), "vault.unlock", "ui", "vault", "primary", "success", "vault unlocked via dashboard")
	}

	return c.JSON(VaultStatusResponse{
		IsUnlocked:     v.IsUnlocked(),
		KeyFingerprint: v.KeyFingerprint(),
	})
}

// VaultLock locks the vault and wipes the key from memory.
func VaultLock(c *fiber.Ctx) error {
	v := vault.GetInstance()
	v.Lock()

	if db := config.GetDatabase(); db != nil {
		_ = db.WriteAuditLog(c.Context(), "vault.lock", "ui", "vault", "primary", "success", "vault locked via dashboard")
	}

	return c.JSON(VaultStatusResponse{
		IsUnlocked: v.IsUnlocked(),
	})
}

// VaultStatus returns the current vault state.
func VaultStatus(c *fiber.Ctx) error {
	v := vault.GetInstance()
	return c.JSON(VaultStatusResponse{
		IsUnlocked:     v.IsUnlocked(),
		KeyFingerprint: v.KeyFingerprint(),
	})
}

// VaultEncrypt encrypts files in the specified path using the vault key.
type VaultEncryptInput struct {
	Path string `json:"path"`
}

// VaultEncrypt encrypts files in the specified path using the vault key.
func VaultEncrypt(c *fiber.Ctx) error {
	var input VaultEncryptInput
	if err := c.BodyParser(&input); err != nil || input.Path == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "path is required",
		})
	}

	v := vault.GetInstance()
	if !v.IsUnlocked() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "vault is locked, unlock first",
		})
	}

	outputPath, err := v.EncryptFile(input.Path)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if db := config.GetDatabase(); db != nil {
		_ = db.WriteAuditLog(c.Context(), "vault.encrypt", "ui", "file", outputPath, "success", "file encrypted via vault")
	}

	return c.JSON(fiber.Map{
		"status":     "encrypted",
		"outputPath": outputPath,
	})
}

// VaultDecrypt decrypts a file using the vault key.
type VaultDecryptInput struct {
	Path string `json:"path"`
}

// VaultDecrypt decrypts a file using the vault key.
func VaultDecrypt(c *fiber.Ctx) error {
	var input VaultDecryptInput
	if err := c.BodyParser(&input); err != nil || input.Path == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "path is required",
		})
	}

	v := vault.GetInstance()
	if !v.IsUnlocked() {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "vault is locked, unlock first",
		})
	}

	outputPath, err := v.DecryptFile(input.Path)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	if db := config.GetDatabase(); db != nil {
		_ = db.WriteAuditLog(c.Context(), "vault.decrypt", "ui", "file", outputPath, "success", "file decrypted via vault")
	}

	return c.JSON(fiber.Map{
		"status":     "decrypted",
		"outputPath": outputPath,
	})
}
