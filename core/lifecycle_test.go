package core_test

import (
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/graph"
)

// The embedded source (parsed, no imports involved) is the ground truth
// for which workflows exist: every workflow it declares must be registered
// in the core DBOS at Launch. A mismatch means workflows/zz_generated.go
// is stale — a new subpackage was added without running:
// go generate ./workflows.
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
