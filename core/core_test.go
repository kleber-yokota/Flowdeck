package core

import (
	"context"
	"maps"
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

// bootCoreForTest resets the singleton and the registry, boots a fresh core
// on the given database URL, and guarantees (in cleanup, LIFO order):
//   - Shutdown before the boot context is canceled (DBOS uses it at Launch);
//   - the registry is restored, so test registrations (M3) never leak into
//     other tests, even with -count=2 or -shuffle.
func bootCoreForTest(t *testing.T, dbURL string) {
	t.Helper()

	mu.Lock()
	savedRegs, savedNames := registrations, maps.Clone(registeredNames)
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		registrations, registeredNames = savedRegs, savedNames
		mu.Unlock()
	})

	Shutdown(5 * time.Second) // nil-safe reset: this test starts clean

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	// The context must stay alive until after Shutdown (DBOS uses it at Launch),
	// so cancel it in cleanup, not when the helper returns.
	t.Cleanup(cancel)

	if err := Boot(ctx, "flowdeck_test", dbURL); err != nil {
		t.Fatalf("boot: %v", err)
	}
	t.Cleanup(func() { Shutdown(5 * time.Second) })
}

// bootSQLiteCoreForTest boots the core on a temporary SQLite database —
// no Postgres needed for the fast test cycle.
func bootSQLiteCoreForTest(t *testing.T) {
	t.Helper()
	bootCoreForTest(t, "sqlite:"+filepath.Join(t.TempDir(), "core-test.db"))
}

// register() must record the workflow so Launch applies it to the DBOS
// context and List() returns it — the web listing depends on this.
func TestRegisterAddsWorkflowToList(t *testing.T) {
	bootSQLiteCoreForTest(t)

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

// register() must fail fast (panic) on a duplicate name — a programming
// error that would otherwise surface far away, inside dbos.RegisterWorkflow.
func TestRegisterDuplicateNamePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate register()")
		}
	}()
	register("hello-world", echoTestWorkflow) // init() already registered it
}

// Boot is a singleton: a second Boot in the same cycle must fail.
func TestBootTwiceErrors(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Boot(context.Background(), "flowdeck_test", ""); err == nil {
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
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("first launch: %v", err)
	}
	if err := Launch(); err == nil {
		t.Fatal("expected error on second Launch (singleton)")
	}
}

// List before Boot must return nil (core not booted), not panic.
func TestListBeforeBootReturnsNil(t *testing.T) {
	Shutdown(5 * time.Second) // nil-safe reset: make sure the core is down

	if got := List(); got != nil {
		t.Fatalf("List before Boot: got %v, want nil", got)
	}
}

// List after Shutdown must return nil (core torn down), not panic.
func TestListAfterShutdownReturnsNil(t *testing.T) {
	bootSQLiteCoreForTest(t)

	Shutdown(5 * time.Second) // explicit: core is torn down before the test ends

	if got := List(); got != nil {
		t.Fatalf("List after Shutdown: got %v, want nil", got)
	}
}
