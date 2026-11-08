package boot_test

import (
	"os"
	"testing"

	"flowdeck/core/graph"
	"flowdeck/core/registry"
	"flowdeck/core/testcore"
)

// The parsed source is the ground truth for which workflows exist: the
// testworkflows source declares exactly the test workflows, and the
// registry holds exactly the registered ones — the two must match. A
// mismatch means a fixture is registered without a func in the source, or
// a func in the source is not registered in RegisterTestWorkflows.
func TestRegistrationMatchesSource(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	g, err := graph.New(os.DirFS("../testworkflows"))
	if err != nil {
		t.Fatalf("build graph from source: %v", err)
	}
	registered := map[string]bool{}
	for _, w := range registry.List() {
		registered[w.Name] = true
	}
	for name := range g.ByName {
		if !registered[name] {
			t.Fatalf("source declares workflow %q but the registry does not — register it in RegisterTestWorkflows", name)
		}
	}
	for name := range registered {
		if g.Of(name) == nil {
			t.Fatalf("registry holds workflow %q but the source does not declare it — the fixture func is missing from testcore", name)
		}
	}
}
