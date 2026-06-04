package config

import (
	"os"
	"strconv"
)

type Config struct {
	AppAddr           string
	DatabaseURL       string
	UploadStoragePath string
	MaxChunkSizeBytes int
}

func Load() Config {
	cfg := Config{
		AppAddr:           envOrDefault("APP_ADDR", ":3001"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		UploadStoragePath: envOrDefault("UPLOAD_STORAGE_PATH", "data/uploads"),
		MaxChunkSizeBytes: envIntOrDefault("MAX_CHUNK_SIZE_BYTES", 2*1024*1024),
	}

	return cfg
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}

func envIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}

	return parsed
}
