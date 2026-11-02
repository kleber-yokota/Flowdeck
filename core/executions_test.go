package core_test

import (
	"flowdeck/core"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/status"
	"testing"
)

// Executions lists the runs of a workflow (phase 3, LIST), newest first:
// the run I enqueued is in the list, with its final state (the assertions
// filter by id — the test database may hold runs from other tests).
func TestExecutionsListsRunsOfWorkflow(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := core.Enqueue("hello-world", "one")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	list, err := core.Executions("hello-world")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	found := false
	for _, e := range list {
		if e.ID != ex.ID {
			continue
		}
		found = true
		if e.State != status.Success {
			t.Fatalf("execution %s state: got %s, want success", e.ID, e.State)
		}
	}
	if !found {
		t.Fatalf("run %s not in Executions(hello-world)", ex.ID)
	}
}

// Executions of a name with no runs is empty (no error).
func TestExecutionsUnknownWorkflowIsEmpty(t *testing.T) {
	testcore.NeedCore(t)
	list, err := core.Executions("no-such-workflow")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("Executions(unknown): got %d rows, want 0", len(list))
	}
}
