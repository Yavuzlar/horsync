package vault

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"horsync/internal/models"
)

func TestVaultSingleton(t *testing.T) {
	v1 := GetInstance()
	v2 := GetInstance()
	if v1 != v2 {
		t.Error("GetInstance should return the same instance")
	}
}

func TestVaultLockUnlock(t *testing.T) {
	v := GetInstance()

	// Should start locked
	if v.IsUnlocked() {
		t.Error("Vault should be locked initially")
	}

	// Unlock with passphrase
	if err := v.Unlock("test-passphrase-123"); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}
	if !v.IsUnlocked() {
		t.Error("Vault should be unlocked after Unlock")
	}

	// Key fingerprint should be non-empty
	fp := v.KeyFingerprint()
	if fp == "" {
		t.Error("KeyFingerprint should be non-empty after unlock")
	}

	// Lock
	v.Lock()
	if v.IsUnlocked() {
		t.Error("Vault should be locked after Lock")
	}

	// Key fingerprint should be empty after lock
	if v.KeyFingerprint() != "" {
		t.Error("KeyFingerprint should be empty after lock")
	}
}

func TestVaultUnlockMultipleTimes(t *testing.T) {
	v := GetInstance()
	v.Lock()

	if err := v.Unlock("passphrase"); err != nil {
		t.Fatalf("First unlock failed: %v", err)
	}

	// Second unlock should be no-op (already unlocked)
	if err := v.Unlock("different-passphrase"); err != nil {
		t.Fatalf("Second unlock should not error: %v", err)
	}
	if !v.IsUnlocked() {
		t.Error("Vault should still be unlocked")
	}

	v.Lock()
}

func TestVaultLockWhenAlreadyLocked(t *testing.T) {
	v := GetInstance()
	v.Lock()
	v.Lock() // Should not panic
	if v.IsUnlocked() {
		t.Error("Vault should be locked")
	}
}

func TestVaultUnlockShortPassphrase(t *testing.T) {
	v := GetInstance()
	v.Lock()

	if err := v.Unlock("abc"); err == nil {
		t.Error("Unlock with passphrase < 4 chars should error")
	}

	if v.IsUnlocked() {
		t.Error("Vault should remain locked after short passphrase")
	}
}

func TestEncryptDecryptFile(t *testing.T) {
	v := GetInstance()
	v.Lock()

	if err := v.Unlock("test-key-12345"); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	// Create a temp file
	tmpDir := t.TempDir()
	originalContent := []byte("This is sensitive data that should be encrypted!")
	originalPath := filepath.Join(tmpDir, "secret.txt")
	if err := os.WriteFile(originalPath, originalContent, 0o644); err != nil {
		t.Fatalf("Write temp file: %v", err)
	}

	// Encrypt
	encryptedPath, err := v.EncryptFile(originalPath)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Encrypted file should exist and be different from original
	encryptedContent, err := os.ReadFile(encryptedPath)
	if err != nil {
		t.Fatalf("Read encrypted file: %v", err)
	}
	if string(encryptedContent) == string(originalContent) {
		t.Error("Encrypted content should differ from original")
	}
	if len(encryptedContent) <= len(originalContent) {
		t.Error("Encrypted file should be larger (nonce + ciphertext)")
	}

	// Decrypt
	decryptedPath, err := v.DecryptFile(encryptedPath)
	if err != nil {
		t.Fatalf("DecryptFile failed: %v", err)
	}

	decryptedContent, err := os.ReadFile(decryptedPath)
	if err != nil {
		t.Fatalf("Read decrypted file: %v", err)
	}

	if string(decryptedContent) != string(originalContent) {
		t.Errorf("Decrypted content does not match original.\nGot:  %q\nWant: %q", string(decryptedContent), string(originalContent))
	}
}

func TestEncryptDecryptWithDifferentKeys(t *testing.T) {
	v := GetInstance()
	v.Lock()

	// Encrypt with one key
	if err := v.Unlock("key-one"); err != nil {
		t.Fatalf("Unlock failed: %v", err)
	}

	tmpDir := t.TempDir()
	content := []byte("Hello, World!")
	filePath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("Write temp file: %v", err)
	}

	encPath, err := v.EncryptFile(filePath)
	if err != nil {
		t.Fatalf("EncryptFile failed: %v", err)
	}

	// Lock and re-unlock with different key
	v.Lock()
	if err := v.Unlock("key-two"); err != nil {
		t.Fatalf("Second unlock failed: %v", err)
	}

	// Decrypting with wrong key should fail
	_, err = v.DecryptFile(encPath)
	if err == nil {
		t.Error("DecryptFile with wrong key should fail")
	}
}

func TestEncryptWhenLocked(t *testing.T) {
	v := GetInstance()
	v.Lock()

	_, err := v.EncryptFile("/tmp/nonexistent")
	if err == nil {
		t.Error("EncryptFile when vault is locked should error")
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Errorf("Error should mention locked, got: %v", err)
	}
}

func TestDecryptWhenLocked(t *testing.T) {
	v := GetInstance()
	v.Lock()

	_, err := v.DecryptFile("/tmp/nonexistent")
	if err == nil {
		t.Error("DecryptFile when vault is locked should error")
	}
	if !strings.Contains(err.Error(), "locked") {
		t.Errorf("Error should mention locked, got: %v", err)
	}
}

func TestVaultLogs(t *testing.T) {
	v := GetInstance()
	v.Lock()

	// Clear logs by getting a fresh perspective
	initialLogs := v.GetLogs()
	initialCount := len(initialLogs)

	if err := v.Unlock("test-logs"); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	v.Lock()

	logs := v.GetLogs()
	if len(logs) <= initialCount {
		t.Error("Logs should have grown after unlock+lock operations")
	}

	// Verify log entries
	foundUnlock := false
	foundLock := false
	for _, log := range logs {
		if log.Event == "vault.unlocked" {
			foundUnlock = true
		}
		if log.Event == "vault.locked" {
			foundLock = true
		}
	}
	if !foundUnlock {
		t.Error("Expected vault.unlocked log entry")
	}
	if !foundLock {
		t.Error("Expected vault.locked log entry")
	}
}

func TestVaultLogTrim(t *testing.T) {
	v := GetInstance()
	v.Lock()

	// Generate more than 100 log entries
	if err := v.Unlock("test"); err != nil {
		t.Fatalf("unlock failed: %v", err)
	}
	for i := 0; i < 120; i++ {
		v.log(createTestLog("test.event"))
	}

	logs := v.GetLogs()
	if len(logs) > 100 {
		t.Errorf("Logs should be capped at 100, got %d", len(logs))
	}
}

func createTestLog(event string) models.SecurityLog {
	return models.SecurityLog{
		Event:  event,
		Type:   "test",
		Time:   "2024-01-01T00:00:00Z",
		Detail: "test log entry",
	}
}
