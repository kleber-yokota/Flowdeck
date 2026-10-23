package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
	_ "github.com/dbos-inc/dbos-transact-golang/dbos/driver/sqlite"
)

// Test-only workflow, registered via register() to exercise the mechanism.
func echoTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	return "pong:" + msg, nil
}

// bootCoreForTest resets the singleton and boots a fresh core on a
// temporary SQLite database (no Postgres needed).
func bootCoreForTest(t *testing.T) {
	t.Helper()

	Shutdown(5 * time.Second) // nil-safe reset: this test starts clean

	dbURL := "sqlite:" + filepath.Join(t.TempDir(), "core-test.db")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// The context must stay alive until after Shutdown (DBOS uses it at Launch),
	// so cancel it in cleanup, not when the helper returns.
	t.Cleanup(cancel)

	if err := Boot(ctx, "flowdeck_core_test", dbURL); err != nil {
		t.Fatalf("boot: %v", err)
	}
	t.Cleanup(func() { Shutdown(5 * time.Second) })
}

// register() must record the workflow so Launch applies it to the DBOS
// context and List() returns it — the web listing depends on this.
func TestRegisterAddsWorkflowToList(t *testing.T) {
	bootCoreForTest(t)

	register("extra-test", echoTestWorkflow)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	names := map[string]bool{}
	for _, w := range List() {
		names[w.Name] = true
	}
	if !names["extra-test"] {
		t.Fatalf("register() did not register: List() missing extra-test: %v", names)
	}
	if !names["hello-world"] {
		t.Fatalf("List() missing hello-world: %v", names)
	}

	handle, err := dbos.RunWorkflow(dbosCtx, echoTestWorkflow, "ping")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if result != "pong:ping" {
		t.Fatalf("result: got %q want %q", result, "pong:ping")
	}
}

// Boot is a singleton: a second Boot in the same cycle must fail.
func TestBootTwiceErrors(t *testing.T) {
	bootCoreForTest(t)

	if err := Boot(context.Background(), "flowdeck_core_test", ""); err == nil {
		t.Fatal("expected error on second Boot (singleton)")
	}
}

// Launch without a prior Boot must fail.
func TestLaunchWithoutBootErrors(t *testing.T) {
	Shutdown(5 * time.Second)

	if err := Launch(); err == nil {
		t.Fatal("expected error on Launch without Boot")
	}
}

// Launch is a singleton: a second Launch in the same cycle must fail.
func TestLaunchTwiceErrors(t *testing.T) {
	bootCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("first launch: %v", err)
	}
	if err := Launch(); err == nil {
		t.Fatal("expected error on second Launch (singleton)")
	}
}
