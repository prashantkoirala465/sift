package config

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/prashantkoirala465/sift/internal/security"
)

func validKey() string {
	return base64.StdEncoding.EncodeToString(make([]byte, security.KeySize))
}

// setMinimalRequiredEnv sets exactly the env vars Load requires, so each
// test can layer its own scenario on top without repeating the baseline.
func setMinimalRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("SIFT_DATABASE_URL", "postgres://localhost/sift")
	t.Setenv("SIFT_ENCRYPTION_KEY", validKey())
}

func TestLoadRequiresDatabaseURL(t *testing.T) {
	t.Setenv("SIFT_ENCRYPTION_KEY", validKey())

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when SIFT_DATABASE_URL is unset")
	}
}

func TestLoadRequiresEncryptionKey(t *testing.T) {
	t.Setenv("SIFT_DATABASE_URL", "postgres://localhost/sift")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error when SIFT_ENCRYPTION_KEY is unset")
	}
}

func TestLoadRejectsInvalidEncryptionKey(t *testing.T) {
	t.Setenv("SIFT_DATABASE_URL", "postgres://localhost/sift")
	t.Setenv("SIFT_ENCRYPTION_KEY", "not-valid-base64-or-the-right-length")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for a malformed encryption key")
	}
}

func TestLoadRejectsInvalidSyncInterval(t *testing.T) {
	setMinimalRequiredEnv(t)
	t.Setenv("SIFT_SYNC_INTERVAL", "not-a-duration")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for an unparseable SIFT_SYNC_INTERVAL")
	}
}

func TestLoadDefaults(t *testing.T) {
	setMinimalRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want default %q", cfg.Addr, ":8080")
	}
	if cfg.SyncInterval != 5*time.Minute {
		t.Errorf("SyncInterval = %v, want default %v", cfg.SyncInterval, 5*time.Minute)
	}
	if len(cfg.EncryptionKey) != security.KeySize {
		t.Errorf("EncryptionKey length = %d, want %d", len(cfg.EncryptionKey), security.KeySize)
	}
}

func TestLoadOverridesFromEnv(t *testing.T) {
	setMinimalRequiredEnv(t)
	t.Setenv("SIFT_ADDR", ":9090")
	t.Setenv("SIFT_SYNC_INTERVAL", "90s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, ":9090")
	}
	if cfg.SyncInterval != 90*time.Second {
		t.Errorf("SyncInterval = %v, want %v", cfg.SyncInterval, 90*time.Second)
	}
}

func TestGoogleConfigured(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want bool
	}{
		{"all three set", Config{GoogleClientID: "id", GoogleClientSecret: "secret", GoogleRedirectURL: "url"}, true},
		{"missing client id", Config{GoogleClientSecret: "secret", GoogleRedirectURL: "url"}, false},
		{"missing secret", Config{GoogleClientID: "id", GoogleRedirectURL: "url"}, false},
		{"missing redirect url", Config{GoogleClientID: "id", GoogleClientSecret: "secret"}, false},
		{"none set", Config{}, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.GoogleConfigured(); got != tc.want {
				t.Errorf("GoogleConfigured() = %v, want %v", got, tc.want)
			}
		})
	}
}
