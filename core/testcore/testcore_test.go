package testcore_test

import (
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/testcore"
	"flowdeck/core/testworkflows"
)

// TestBootTestBootsAndCleans is the per-test boot contract: BootTest
// boots the test core (app flowdeck_test, a fresh sqlite file under the
// test's TempDir), registers the test workflows on it and launches the
// executor — the returned context is live, and the Shutdown (the t.Cleanup
// BootTest registered) tears the core down again.
func TestBootTestBootsAndCleans(t *testing.T) {
	c := testcore.BootTest(t, "../testworkflows")
	if c == nil {
		t.Fatal("BootTest returned a nil context, want the live context")
	}
}

// TestBootTestNamedSubset is the register route: the named workflows
// (a subset of the ten) register on the booted core plus the frozen
// fixture queues — no name table.
func TestBootTestNamedSubset(t *testing.T) {
	c := testcore.BootTest(t, "../testworkflows", testworkflows.ByName["live-test"], testworkflows.ByName["fork-test"])
	names := map[string]bool{}
	for _, w := range dbos.ListRegisteredWorkflows(c) {
		names[w.Name] = true
	}
	if len(names) != 2 || !names["live-test"] || !names["fork-test"] {
		t.Fatalf("registered workflows: got %v, want exactly live-test · fork-test", names)
	}
}
