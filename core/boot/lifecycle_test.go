package boot_test

import (
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/graph"

	// The workflow files register themselves in the env-driven DBOS
	// context (their init) — this test asserts the registry matches the
	// source.
	_ "flowdeck/workflows"
)

// The embedded source (parsed, no imports involved) is the ground truth
// for which workflows exist: every workflow it declares must be registered
// in the core DBOS at Launch. A mismatch means workflows/zz_generated.go
// is stale — a new subpackage was added without running:
// go generate ./workflows.
// needCore skips the test when the env-driven context is absent (the env
// vars were not exported before the test binary started).
func needCore(t *testing.T) {
	t.Helper()
	if boot.Ctx() == nil {
		t.Skip("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — source .env.test")
	}
}

func TestRegistrationMatchesSource(t *testing.T) {
	needCore(t)
	g, err := graph.New(os.DirFS("../workflows"))
	if err != nil {
		t.Fatalf("build graph from source: %v", err)
	}
	registered := map[string]bool{}
	for _, w := range core.List() {
		registered[w.Name] = true
	}
	for name := range g.ByName {
		if !registered[name] {
			t.Fatalf("source declares workflow %q but the registry does not — run 'go generate ./workflows'", name)
		}
	}
}
