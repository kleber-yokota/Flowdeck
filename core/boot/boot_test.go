package boot_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"flowdeck/core/boot"
	"flowdeck/core/testworkflows"
)

// TestBootExplicit is the explicit boot on a fresh sqlite file: Boot
// creates the context (Ctx() non-nil), Launch and Ready on it, and
// Shutdown resets the singleton (Ctx() nil).
func TestBootExplicit(t *testing.T) {
	if err := boot.Boot(context.Background(), "flowdeck_test",
		"sqlite:"+filepath.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	if boot.Ctx() == nil {
		t.Fatal("Ctx() after Boot: want the live context")
	}
	// The production order: Boot → Register → Launch — Launch compares the
	// source (set by TestMain) against the registry.
	testworkflows.RegisterTestWorkflows(boot.Ctx())

	if err := boot.Launch(); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if err := boot.Ready(); err != nil {
		t.Fatalf("Ready: %v", err)
	}
	boot.Shutdown(5 * time.Second)
	if boot.Ctx() != nil {
		t.Fatal("Ctx() after Shutdown: want nil")
	}
}
