package registry_test

import (
	"os"
	"testing"

	"flowdeck/core/internal/testcore"
	"flowdeck/core/registry"

	// The workflow files register themselves in the env-driven DBOS
	// context (their init) — the list is their init's result.
	_ "flowdeck/workflows"
)

// TestMain brings the env-driven core up for the tests: the paths graph
// from the workflows tree on disk, and the executor when the env context
// exists (the mock tests run without it).
func TestMain(m *testing.M) {
	testcore.Setup("../../workflows")
	os.Exit(m.Run())
}

// The registry (the web listing's source) has the shipped workflows — the
// registration is the workflow files' init, applied to the env context.
func TestListShowsRegisteredWorkflows(t *testing.T) {
	testcore.NeedCore(t)
	names := map[string]bool{}
	for _, w := range registry.List() {
		names[w.Name] = true
	}
	for _, want := range []string{"hello-world", "pipeline", "decide", "buy", "pause"} {
		if !names[want] {
			t.Fatalf("registry missing %s: %v", want, names)
		}
	}
}
