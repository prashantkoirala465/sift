// Package config loads runtime configuration from the environment. Sift is
// meant to be self-hosted with a single binary and a handful of env vars —
// no config file format, no flags for anything that belongs in the
// environment.
package config

import (
	"fmt"
	"os"
)

type Config struct {
	// Addr is the host:port the HTTP server listens on.
	Addr string

	// DatabaseURL is a Postgres connection string.
	DatabaseURL string
}

func Load() (Config, error) {
	cfg := Config{
		Addr:        getEnv("SIFT_ADDR", ":8080"),
		DatabaseURL: os.Getenv("SIFT_DATABASE_URL"),
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("SIFT_DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
