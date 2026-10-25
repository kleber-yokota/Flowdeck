package main

import (
	"testing"
	"time"
)

// loadConfig must fail fast when the required env vars are missing (the core
// reads them in its own init — this is the early-error twin).
func TestLoadConfigRequiresEnv(t *testing.T) {
	t.Setenv("DBOS_APP_NAME", "")
	t.Setenv("DBOS_SYSTEM_DATABASE_URL", "")
	if _, err := loadConfig(); err == nil {
		t.Fatal("expected error when DBOS_APP_NAME / DBOS_SYSTEM_DATABASE_URL are missing")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	t.Setenv("DBOS_APP_NAME", "flowdeck")
	t.Setenv("DBOS_SYSTEM_DATABASE_URL", "postgres://db:5432/flowdeck")
	t.Setenv("FLOWDECK_HTTP_ADDR", "")
	t.Setenv("FLOWDECK_HTTP_TIMEOUT", "")
	t.Setenv("FLOWDECK_CORE_TIMEOUT", "")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.httpAddr != ":8080" {
		t.Fatalf("default addr: got %q want %q", cfg.httpAddr, ":8080")
	}
	if cfg.httpTimeout != 5*time.Second {
		t.Fatalf("default httpTimeout: got %v", cfg.httpTimeout)
	}
	if cfg.coreTimeout != 5*time.Second {
		t.Fatalf("default coreTimeout: got %v", cfg.coreTimeout)
	}
}

func TestLoadConfigEnvOverrides(t *testing.T) {
	t.Setenv("DBOS_APP_NAME", "flowdeck")
	t.Setenv("DBOS_SYSTEM_DATABASE_URL", "postgres://db:5432/flowdeck")
	t.Setenv("FLOWDECK_HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("FLOWDECK_HTTP_TIMEOUT", "2s")
	t.Setenv("FLOWDECK_CORE_TIMEOUT", "3s")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.httpAddr != "127.0.0.1:9000" {
		t.Fatalf("addr: got %q", cfg.httpAddr)
	}
	if cfg.httpTimeout != 2*time.Second {
		t.Fatalf("httpTimeout: got %v", cfg.httpTimeout)
	}
	if cfg.coreTimeout != 3*time.Second {
		t.Fatalf("coreTimeout: got %v", cfg.coreTimeout)
	}
}

// An invalid duration must fall back to the default (the process stays up).
func TestLoadConfigInvalidDurationFallsBack(t *testing.T) {
	t.Setenv("DBOS_APP_NAME", "flowdeck")
	t.Setenv("DBOS_SYSTEM_DATABASE_URL", "postgres://db:5432/flowdeck")
	t.Setenv("FLOWDECK_HTTP_TIMEOUT", "not-a-duration")

	cfg, err := loadConfig()
	if err != nil {
		t.Fatalf("loadConfig: %v", err)
	}
	if cfg.httpTimeout != 5*time.Second {
		t.Fatalf("httpTimeout fallback: got %v want 5s", cfg.httpTimeout)
	}
}
