package reads_test

import (
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// Executions lists the runs of a workflow, newest first: the run the test
// enqueued is the one row on the fresh per-test file, with its final
// state.
func TestExecutionsListsRunsOfWorkflow(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("live-test", "one")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	list, err := reads.Executions("live-test")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("Executions(live-test): got %d runs, want exactly the one enqueued", len(list))
	}
	if list[0].ID != ex.ID {
		t.Fatalf("Executions: got run %s, want %s", list[0].ID, ex.ID)
	}
	if list[0].State != status.Success {
		t.Fatalf("execution %s state: got %s, want success", list[0].ID, list[0].State)
	}
}

// Executions of a name with no runs is empty (no error).
func TestExecutionsUnknownWorkflowIsEmpty(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	list, err := reads.Executions("no-such-workflow")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("Executions(unknown): got %d rows, want 0", len(list))
	}
}
