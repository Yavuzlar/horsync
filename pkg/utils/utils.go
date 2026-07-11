package utils

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"time"
)

// GetLocalIP returns the preferred local network IPv4 address.
// It excludes virtual network adapter ranges (WSL, VMnet, APIPA) to
// identify the main LAN network IP.
func GetLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "127.0.0.1"
	}

	// First pass: return the first non-loopback, non-virtual IPv4 address
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ipStr := ipnet.IP.String()
				if !isVirtualIP(ipStr) {
					return ipStr
				}
			}
		}
	}

	// Second pass: fallback to first non-APIPA address
	for _, address := range addrs {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				ipStr := ipnet.IP.String()
				if !strings.HasPrefix(ipStr, "169.254.") {
					return ipStr
				}
			}
		}
	}

	return "127.0.0.1"
}

// isVirtualIP checks if an IP belongs to a known virtual network adapter range.
func isVirtualIP(ip string) bool {
	prefixes := []string{
		"192.168.217.", // Common WSL / VMware
		"192.168.111.", // Common VMware
		"172.",         // WSL / Docker virtual adapters
		"169.254.",     // APIPA / link-local
	}
	for _, p := range prefixes {
		if strings.HasPrefix(ip, p) {
			return true
		}
	}
	return false
}

// TruncateString shortens a string to the given max length, appending "..." if truncated.
func TruncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// ==========================================
// Shared Formatting Utilities
// ==========================================

// FormatUptime converts total seconds into a human-readable "Xd YYh" format.
func FormatUptime(totalSeconds int64) string {
	if totalSeconds <= 0 {
		return "0D 00H"
	}

	duration := time.Duration(totalSeconds) * time.Second
	days := duration / (24 * time.Hour)
	duration -= days * 24 * time.Hour
	hours := duration / time.Hour

	return fmt.Sprintf("%dD %02dH", days, hours)
}

// FormatLastSeen converts a time pointer into a relative time string.
func FormatLastSeen(value *time.Time, awaitingText string) string {
	if value == nil {
		if awaitingText == "" {
			awaitingText = "Awaiting approval"
		}
		return awaitingText
	}

	elapsed := time.Since(value.UTC())
	switch {
	case elapsed < time.Minute:
		return "Just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
}

// PreviewFingerprint returns the first 12 characters of a fingerprint hash.
func PreviewFingerprint(value string) string {
	if len(value) <= 12 {
		return value
	}
	return value[:12]
}

// HashSHA256 returns the lowercase hex SHA-256 hash of a string.
func HashSHA256(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// GenerateRandomHex returns a cryptographically random hex string of n bytes.
func GenerateRandomHex(n int) (string, error) {
	raw := make([]byte, n)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate random: %w", err)
	}
	return hex.EncodeToString(raw), nil
}

// NormalizeValue returns the trimmed string, or a fallback if empty.
func NormalizeValue(value string, fallback string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback
	}
	return trimmed
}

// FormatBytes converts a byte count to a human-readable string (B, KB, MB, GB).
func FormatBytes(totalSize int64) string {
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)

	switch {
	case totalSize >= gb:
		return fmt.Sprintf("%.1f GB", float64(totalSize)/gb)
	case totalSize >= mb:
		return fmt.Sprintf("%.1f MB", float64(totalSize)/mb)
	case totalSize >= kb:
		return fmt.Sprintf("%.1f KB", float64(totalSize)/kb)
	default:
		return fmt.Sprintf("%d B", totalSize)
	}
}

// FormatRelativeTime returns a human-readable relative time string.
func FormatRelativeTime(t time.Time) string {
	elapsed := time.Since(t.UTC())
	switch {
	case elapsed < time.Minute:
		return "Just now"
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm ago", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(elapsed.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(elapsed.Hours()/24))
	}
}
