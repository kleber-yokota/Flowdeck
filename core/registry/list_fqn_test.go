package registry_test

import (
	"errors"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/registry"
	"flowdeck/core/testcore"
	"flowdeck/core/testworkflows"
)

// The List name fallback: a workflow registered without the
// WithWorkflowName option has no entry name — the List exposes it by its
// FQN (the function's full name).

// unamedWf is a workflow registered without a workflow-name option.
func unamedWf(ctx dbos.Context, msg string) (string, error) {
	return msg, nil
}

// A workflow registered without a name is listed by its FQN (the function's
// full name), alongside the named fixtures.
func TestListUnamedWorkflowUsesTheFQN(t *testing.T) {
	testcore.BootTest(t, "../testworkflows", func(c dbos.Context) {
		testworkflows.RegisterTestWorkflows(c)
		dbos.RegisterWorkflow(c, unamedWf)
	})
	names := map[string]bool{}
	for _, w := range registry.List() {
		names[w.Name] = true
	}
	if !names["flowdeck/core/registry_test.unamedWf"] {
		t.Errorf("List: missing the FQN of the unamed workflow: %v", names)
	}
	if len(names) != 11 {
		t.Errorf("List: got %d workflows, want 11 (the 10 fixtures + the unamed one): %v", len(names), names)
	}
}

// After a Shutdown the registry reads the unbooted state: the List is nil
// and the Spec of any name is the unknown-workflow error (the graph is
// reset with the context).
func TestRegistryAfterShutdown(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	boot.Shutdown(time.Second)
	if got := registry.List(); got != nil {
		t.Errorf("List(shutdown): got %v, want nil", got)
	}
	if _, err := registry.Spec("fork-test"); !errors.Is(err, registry.ErrUnknownWorkflow) {
		t.Errorf("Spec(shutdown): got %v, want ErrUnknownWorkflow", err)
	}
}
