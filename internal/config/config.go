// Package config loads runtime configuration from the environment. Sift is
// meant to be self-hosted with a single binary and a handful of env vars —
// no config file format, no flags for anything that belongs in the
// environment.
package config

import (
	"fmt"
	"os"
	"time"

	"github.com/prashantkoirala465/sift/internal/security"
)

type Config struct {
	// Addr is the host:port the HTTP server listens on.
	Addr string

	// DatabaseURL is a Postgres connection string.
	DatabaseURL string

	// EncryptionKey is a base64-encoded 32-byte AES-256 key used to encrypt
	// secrets (OAuth tokens) before they're stored. Required.
	EncryptionKey []byte

	// Google* configure the Gmail OAuth flow. Optional at startup -- the
	// rest of Sift works without them -- but required before /auth/google
	// can do anything.
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string

	// SyncInterval is how often the Gmail sync worker ticks.
	SyncInterval time.Duration

	// AnthropicAPIKey enables the LLM classification fallback tier.
	// Optional -- without it, Sift classifies with rules only.
	AnthropicAPIKey string

	// AuthPassword, if set, gates the entire web UI and API behind HTTP
	// Basic Auth. Strongly recommended for anything reachable beyond
	// localhost; optional so local development doesn't need it.
	AuthPassword string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:               getEnv("SIFT_ADDR", ":8080"),
		DatabaseURL:        os.Getenv("SIFT_DATABASE_URL"),
		GoogleClientID:     os.Getenv("SIFT_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("SIFT_GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("SIFT_GOOGLE_REDIRECT_URL"),
		AnthropicAPIKey:    os.Getenv("SIFT_ANTHROPIC_API_KEY"),
		AuthPassword:       os.Getenv("SIFT_AUTH_PASSWORD"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("SIFT_DATABASE_URL is required")
	}

	syncInterval := getEnv("SIFT_SYNC_INTERVAL", "5m")
	interval, err := time.ParseDuration(syncInterval)
	if err != nil {
		return Config{}, fmt.Errorf("SIFT_SYNC_INTERVAL: %w", err)
	}
	cfg.SyncInterval = interval

	keyB64 := os.Getenv("SIFT_ENCRYPTION_KEY")
	if keyB64 == "" {
		return Config{}, fmt.Errorf("SIFT_ENCRYPTION_KEY is required")
	}
	key, err := security.ParseKey(keyB64)
	if err != nil {
		return Config{}, fmt.Errorf("SIFT_ENCRYPTION_KEY: %w", err)
	}
	cfg.EncryptionKey = key

	return cfg, nil
}

// GoogleConfigured reports whether enough Google OAuth config is present to
// build an oauth2.Config.
func (c Config) GoogleConfigured() bool {
	return c.GoogleClientID != "" && c.GoogleClientSecret != "" && c.GoogleRedirectURL != ""
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
