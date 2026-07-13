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
// The PBKDF2 salt is persisted to disk so encrypted files remain decryptable
// across server restarts and process respawns.
type Vault struct {
	mu         sync.RWMutex
	logs       []models.SecurityLog
	masterKey  []byte // Derived key, kept only in memory
	isUnlocked bool
	salt       []byte
	saltPath   string
}

var instance *Vault
var once sync.Once

// saltFilePath is the persistent location of the PBKDF2 salt. The salt itself
// is not secret — it only needs to be stable across server restarts.
const saltFilePath = "data/vault.salt"

// GetInstance returns the singleton vault instance.
func GetInstance() *Vault {
	once.Do(func() {
		instance = &Vault{
			logs:     make([]models.SecurityLog, 0),
			salt:     make([]byte, 32),
			saltPath: saltFilePath,
		}
		// Generate a random ephemeral salt as a fallback. Start() will replace
		// this with the persisted (or newly persisted) salt before use.
		if _, err := rand.Read(instance.salt); err != nil {
			copy(instance.salt, []byte("HORSYNC_VAULT_SALT_2024_DEFAULT"))
		}
	})
	return instance
}

// Start initializes the vault, ensures a persistent PBKDF2 salt is loaded from
// disk (creating one on first boot), and logs a startup event.
func (v *Vault) Start() {
	v.loadOrCreateSalt()

	v.log(models.SecurityLog{
		Event:  "vault.initialized",
		Type:   "info",
		Time:   time.Now().UTC().Format(time.RFC3339),
		Detail: "Zero-knowledge vault engine started. No keys loaded.",
	})
}

// loadOrCreateSalt ensures a stable, persisted salt is available. If a salt
// file exists, it is loaded. Otherwise a new random salt is generated and
// written to disk so subsequent process starts derive the same PBKDF2 key
// from the same passphrase.
func (v *Vault) loadOrCreateSalt() {
	if v.saltPath == "" {
		return
	}

	// Ensure the parent directory exists so the salt file can be written.
	if err := os.MkdirAll(filepath.Dir(v.saltPath), 0o700); err != nil {
		v.log(models.SecurityLog{
			Event:  "vault.salt.persist_failed",
			Type:   "warning",
			Time:   time.Now().UTC().Format(time.RFC3339),
			Detail: "Could not create vault salt directory; using in-memory salt: " + err.Error(),
		})
		return
	}

	// Attempt to load an existing salt.
	if data, err := os.ReadFile(v.saltPath); err == nil && len(data) >= 16 {
		v.salt = data
		v.log(models.SecurityLog{
			Event:  "vault.salt.loaded",
			Type:   "info",
			Time:   time.Now().UTC().Format(time.RFC3339),
			Detail: "Persistent PBKDF2 salt loaded from disk.",
		})
		return
	}

	// Otherwise generate and persist a fresh salt.
	fresh := make([]byte, 32)
	if _, err := rand.Read(fresh); err != nil {
		// Cryptographic random unavailable; reuse the ephemeral fallback salt
		// but do not persist it, otherwise we'd persist a known weak value.
		v.log(models.SecurityLog{
			Event:  "vault.salt.ephemeral",
			Type:   "warning",
			Time:   time.Now().UTC().Format(time.RFC3339),
			Detail: "Could not generate cryptographically random salt; using ephemeral in-memory salt. Encryption is untrusted across restarts.",
		})
		return
	}

	if err := os.WriteFile(v.saltPath, fresh, 0o600); err != nil {
		v.log(models.SecurityLog{
			Event:  "vault.salt.persist_failed",
			Type:   "warning",
			Time:   time.Now().UTC().Format(time.RFC3339),
			Detail: "Could not persist vault salt to disk; using in-memory salt: " + err.Error(),
		})
		v.salt = fresh
		return
	}

	v.salt = fresh
	v.log(models.SecurityLog{
		Event:  "vault.salt.persisted",
		Type:   "info",
		Time:   time.Now().UTC().Format(time.RFC3339),
		Detail: "New PBKDF2 salt generated and persisted to disk.",
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
