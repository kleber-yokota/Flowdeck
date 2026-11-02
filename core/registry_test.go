package core_test

import (
	"testing"

	"flowdeck/core"
	"flowdeck/core/internal/testcore"
)

// The registry (the web listing's source) has the shipped workflows — the
// registration is the workflow files' init, applied to the env context.
func TestListShowsRegisteredWorkflows(t *testing.T) {
	testcore.NeedCore(t)
	names := map[string]bool{}
	for _, w := range core.List() {
		names[w.Name] = true
	}
	for _, want := range []string{"hello-world", "pipeline", "decide", "buy", "pause"} {
		if !names[want] {
			t.Fatalf("registry missing %s: %v", want, names)
		}
	}
}
