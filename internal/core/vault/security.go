package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/crypto/pbkdf2"

	"horsync/internal/models"
)

// Vault provides client-side zero-knowledge encryption using AES-GCM 256-bit.
// Keys are derived from a passphrase using PBKDF2 and never stored in plaintext.
type Vault struct {
	mu         sync.RWMutex
	logs       []models.SecurityLog
	masterKey  []byte // Derived key, kept only in memory
	isUnlocked bool
	salt       []byte
}

var instance *Vault
var once sync.Once

// GetInstance returns the singleton vault instance.
func GetInstance() *Vault {
	once.Do(func() {
		instance = &Vault{
			logs: make([]models.SecurityLog, 0),
			salt: make([]byte, 32),
		}
		// Generate a random salt on initialization
		if _, err := rand.Read(instance.salt); err != nil {
			// Fallback salt if random fails (not cryptographically ideal but prevents nil panic)
			copy(instance.salt, []byte("HORSYNC_VAULT_SALT_2024_DEFAULT"))
		}
	})
	return instance
}

// Start initializes the vault and logs a startup event.
func (v *Vault) Start() {
	v.log(models.SecurityLog{
		Event:  "vault.initialized",
		Type:   "info",
		Time:   time.Now().UTC().Format(time.RFC3339),
		Detail: "Zero-knowledge vault engine started. No keys loaded.",
	})
}

// Stop locks the vault and wipes the master key from memory.
func (v *Vault) Stop() {
	v.mu.Lock()

	// Securely wipe the master key from memory
	for i := range v.masterKey {
		v.masterKey[i] = 0
	}
	v.masterKey = nil
	v.isUnlocked = false
	v.mu.Unlock()

	v.log(models.SecurityLog{
		Event:  "vault.shutdown",
		Type:   "info",
		Time:   time.Now().UTC().Format(time.RFC3339),
		Detail: "Vault engine stopped and keys wiped from memory.",
	})
}

// Unlock derives the AES-256 key from a passphrase using PBKDF2.
func (v *Vault) Unlock(passphrase string) error {
	v.mu.Lock()
	if v.isUnlocked {
		v.mu.Unlock()
		return nil
	}

	if len(passphrase) < 4 {
		v.mu.Unlock()
		return fmt.Errorf("passphrase must be at least 4 characters")
	}

	// Derive 256-bit key using PBKDF2 with 100,000 iterations
	key := pbkdf2.Key([]byte(passphrase), v.salt, 100000, 32, sha256.New)
	v.masterKey = key
	v.isUnlocked = true
	v.mu.Unlock()

	v.log(models.SecurityLog{
		Event:  "vault.unlocked",
		Type:   "unlock",
		Time:   time.Now().UTC().Format(time.RFC3339),
		Detail: "Vault unlocked. Encryption key derived and loaded in local memory.",
	})

	return nil
}

// Lock wipes the derived key from memory.
func (v *Vault) Lock() {
	v.mu.Lock()
	if !v.isUnlocked {
		v.mu.Unlock()
		return
	}

	for i := range v.masterKey {
		v.masterKey[i] = 0
	}
	v.masterKey = nil
	v.isUnlocked = false
	v.mu.Unlock()

	v.log(models.SecurityLog{
		Event:  "vault.locked",
		Type:   "lock",
		Time:   time.Now().UTC().Format(time.RFC3339),
		Detail: "Vault locked. Encryption key wiped from memory.",
	})
}

// IsUnlocked returns whether the vault currently has a key loaded.
func (v *Vault) IsUnlocked() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.isUnlocked
}

// EncryptFile encrypts a file using AES-GCM and writes the ciphertext to a new file.
// The output file path is the input path + ".encrypted".
func (v *Vault) EncryptFile(filePath string) (string, error) {
	v.mu.RLock()
	if !v.isUnlocked || v.masterKey == nil {
		v.mu.RUnlock()
		return "", fmt.Errorf("vault is locked, unlock first")
	}
	key := make([]byte, len(v.masterKey))
	copy(key, v.masterKey)
	v.mu.RUnlock()

	plaintext, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	// Generate random nonce
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}

	// Encrypt and append nonce + ciphertext
	ciphertext := aesGCM.Seal(nonce, nonce, plaintext, nil)

	outputPath := filePath + ".encrypted"
	if err := os.WriteFile(outputPath, ciphertext, 0o644); err != nil {
		return "", fmt.Errorf("write encrypted file: %w", err)
	}

	v.log(models.SecurityLog{
		Event:  "vault.encrypt",
		Type:   "crypto",
		Time:   time.Now().UTC().Format(time.RFC3339),
		Detail: fmt.Sprintf("File encrypted: %s", filepath.Base(filePath)),
	})

	return outputPath, nil
}

// DecryptFile decrypts a file that was encrypted with EncryptFile.
// The output file path strips the ".encrypted" extension.
func (v *Vault) DecryptFile(filePath string) (string, error) {
	v.mu.RLock()
	if !v.isUnlocked || v.masterKey == nil {
		v.mu.RUnlock()
		return "", fmt.Errorf("vault is locked, unlock first")
	}
	key := make([]byte, len(v.masterKey))
	copy(key, v.masterKey)
	v.mu.RUnlock()

	ciphertext, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("read encrypted file: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("create cipher: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("create GCM: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := aesGCM.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("decrypt: %w", err)
	}

	outputPath := filePath
	if len(outputPath) > 10 && outputPath[len(outputPath)-10:] == ".encrypted" {
		outputPath = outputPath[:len(outputPath)-10]
	} else {
		outputPath = outputPath + ".decrypted"
	}

	if err := os.WriteFile(outputPath, plaintext, 0o644); err != nil {
		return "", fmt.Errorf("write decrypted file: %w", err)
	}

	v.log(models.SecurityLog{
		Event:  "vault.decrypt",
		Type:   "crypto",
		Time:   time.Now().UTC().Format(time.RFC3339),
		Detail: fmt.Sprintf("File decrypted: %s", filepath.Base(filePath)),
	})

	return outputPath, nil
}

// KeyFingerprint returns a truncated SHA-256 fingerprint of the loaded key
// (useful for UI display without exposing the full key).
func (v *Vault) KeyFingerprint() string {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if !v.isUnlocked || v.masterKey == nil {
		return ""
	}

	sum := sha256.Sum256(v.masterKey)
	return hex.EncodeToString(sum[:8])
}

// GetLogs returns a copy of all security log entries.
func (v *Vault) GetLogs() []models.SecurityLog {
	v.mu.RLock()
	defer v.mu.RUnlock()

	result := make([]models.SecurityLog, len(v.logs))
	copy(result, v.logs)
	return result
}

func (v *Vault) log(entry models.SecurityLog) {
	v.mu.Lock()
	defer v.mu.Unlock()

	entry.ID = len(v.logs) + 1
	v.logs = append(v.logs, entry)
	if len(v.logs) > 100 {
		v.logs = v.logs[1:]
	}
}
