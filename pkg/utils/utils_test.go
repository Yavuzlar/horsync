package utils

import (
	"strings"
	"testing"
	"time"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input   string
		maxLen  int
		wantLen int
		wantSuffix string
	}{
		{"hello", 10, 5, ""},
		{"hello world", 5, 8, "..."},
		{"", 5, 0, ""},
		{"abc", 3, 3, ""},
	}

	for _, tt := range tests {
		got := TruncateString(tt.input, tt.maxLen)
		if len(got) != tt.wantLen {
			t.Errorf("TruncateString(%q, %d) length = %d, want %d", tt.input, tt.maxLen, len(got), tt.wantLen)
		}
		if tt.wantSuffix != "" && !strings.HasSuffix(got, tt.wantSuffix) {
			t.Errorf("TruncateString(%q, %d) = %q, want suffix %q", tt.input, tt.maxLen, got, tt.wantSuffix)
		}
	}
}

func TestFormatUptime(t *testing.T) {
	tests := []struct {
		seconds int64
		want    string
	}{
		{0, "0D 00H"},
		{-1, "0D 00H"},
		{3600, "0D 01H"},
		{86400, "1D 00H"},
		{90000, "1D 01H"},
		{172800, "2D 00H"},
	}

	for _, tt := range tests {
		got := FormatUptime(tt.seconds)
		if got != tt.want {
			t.Errorf("FormatUptime(%d) = %q, want %q", tt.seconds, got, tt.want)
		}
	}
}

func TestFormatLastSeen(t *testing.T) {
	now := time.Now()

	// Just now (within last minute)
	recent := now.Add(-30 * time.Second)
	got := FormatLastSeen(&recent, "Awaiting approval")
	if got != "Just now" {
		t.Errorf("Expected 'Just now', got %q", got)
	}

	// Minutes ago
	minsAgo := now.Add(-5 * time.Minute)
	got = FormatLastSeen(&minsAgo, "Awaiting approval")
	if !strings.HasSuffix(got, "m ago") {
		t.Errorf("Expected minutes format, got %q", got)
	}

	// Hours ago
	hoursAgo := now.Add(-3 * time.Hour)
	got = FormatLastSeen(&hoursAgo, "Awaiting approval")
	if !strings.HasSuffix(got, "h ago") {
		t.Errorf("Expected hours format, got %q", got)
	}

	// Nil value
	got = FormatLastSeen(nil, "Awaiting approval")
	if got != "Awaiting approval" {
		t.Errorf("Expected 'Awaiting approval' for nil, got %q", got)
	}

	// Nil with custom default text
	got = FormatLastSeen(nil, "Pending activation")
	if got != "Pending activation" {
		t.Errorf("Expected 'Pending activation' for nil, got %q", got)
	}

	// Days ago
	daysAgo := now.Add(-48 * time.Hour)
	got = FormatLastSeen(&daysAgo, "")
	if !strings.HasSuffix(got, "d ago") {
		t.Errorf("Expected days format, got %q", got)
	}
}

func TestPreviewFingerprint(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"abc123", "abc123"},
		{"abcdef1234567890", "abcdef123456"},
		{"", ""},
		{"short", "short"},
	}

	for _, tt := range tests {
		got := PreviewFingerprint(tt.input)
		if got != tt.want {
			t.Errorf("PreviewFingerprint(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestHashSHA256(t *testing.T) {
	// Known SHA-256 values
	empty := HashSHA256("")
	if len(empty) != 64 {
		t.Errorf("Empty hash should be 64 chars, got %d", len(empty))
	}

	hello := HashSHA256("hello")
	if len(hello) != 64 {
		t.Errorf("Hash should be 64 chars, got %d", len(hello))
	}

	// Deterministic
	if HashSHA256("hello") != HashSHA256("hello") {
		t.Error("Hash should be deterministic")
	}

	// Different inputs produce different hashes
	if HashSHA256("hello") == HashSHA256("world") {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestGenerateRandomHex(t *testing.T) {
	// Test with various lengths
	for _, n := range []int{1, 8, 16, 32} {
		result, err := GenerateRandomHex(n)
		if err != nil {
			t.Fatalf("GenerateRandomHex(%d) error: %v", n, err)
		}
		expectedLen := n * 2
		if len(result) != expectedLen {
			t.Errorf("GenerateRandomHex(%d) length = %d, want %d", n, len(result), expectedLen)
		}
	}

	// Two calls should produce different values
	a, _ := GenerateRandomHex(16)
	b, _ := GenerateRandomHex(16)
	if a == b {
		t.Error("Two GenerateRandomHex calls should produce different values")
	}
}

func TestNormalizeValue(t *testing.T) {
	tests := []struct {
		input    string
		fallback string
		want     string
	}{
		{"hello", "default", "hello"},
		{"", "default", "default"},
		{"  ", "fallback", "fallback"},
		{"  trimmed  ", "default", "trimmed"},
	}

	for _, tt := range tests {
		got := NormalizeValue(tt.input, tt.fallback)
		if got != tt.want {
			t.Errorf("NormalizeValue(%q, %q) = %q, want %q", tt.input, tt.fallback, got, tt.want)
		}
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{500, "500 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
	}

	for _, tt := range tests {
		got := FormatBytes(tt.bytes)
		if got != tt.want {
			t.Errorf("FormatBytes(%d) = %q, want %q", tt.bytes, got, tt.want)
		}
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Now()

	// Just now
	got := FormatRelativeTime(now)
	if got != "Just now" {
		t.Errorf("Expected 'Just now', got %q", got)
	}

	// Minutes
	minsAgo := now.Add(-5 * time.Minute)
	got = FormatRelativeTime(minsAgo)
	if !strings.HasSuffix(got, "m ago") {
		t.Errorf("Expected minutes format, got %q", got)
	}

	// Hours
	hoursAgo := now.Add(-3 * time.Hour)
	got = FormatRelativeTime(hoursAgo)
	if !strings.HasSuffix(got, "h ago") {
		t.Errorf("Expected hours format, got %q", got)
	}

	// Days
	daysAgo := now.Add(-48 * time.Hour)
	got = FormatRelativeTime(daysAgo)
	if !strings.HasSuffix(got, "d ago") {
		t.Errorf("Expected days format, got %q", got)
	}
}
