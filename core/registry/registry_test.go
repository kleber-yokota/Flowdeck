package registry_test

import (
	"os"
	"testing"

	"flowdeck/core/internal/testcore"
	"flowdeck/core/registry"
)

// TestMain brings the env-driven core up for the tests: the paths graph
// from the testcore source (the parser fixture) and the executor when the
// env context exists (the unit tests run without it).
func TestMain(m *testing.M) {
	testcore.Setup("../testworkflows")
	os.Exit(m.Run())
}

// The registry (the web listing's source) has exactly the test workflows —
// the registration is RegisterTestWorkflows, applied to the env context.
func TestListShowsRegisteredWorkflows(t *testing.T) {
	testcore.NeedCore(t)
	names := map[string]bool{}
	for _, w := range registry.List() {
		names[w.Name] = true
	}
	for _, want := range []string{"failing-test", "resume-test", "live-test", "noinput-test", "block-test", "fork-test", "struct-test", "linear-test", "inner-test", "child-test"} {
		if !names[want] {
			t.Fatalf("registry missing %s: %v", want, names)
		}
	}
	if len(names) != 10 {
		t.Fatalf("registry has %d workflows, want exactly the 10 test fixtures: %v", len(names), names)
	}
}
