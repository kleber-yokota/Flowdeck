package reads_test

import (
	"errors"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// The executed path of a kitchen run (phase 4, STEP 3): the run's state
// (the poll control) and the eight rows in order (Postgres returns them
// step_id asc), each mapped onto its kind — the step rows (base, side; spin
// — one row per iteration), the wait rows (DBOS.sleep · DBOS.getResult ·
// DBOS.select — the SDK-internal names) and the child row (pipeline — the
// RunWorkflow dispatch: the row's name is the child workflow's name and it
// never completes; the wait row does).
func TestStepsOfKitchenRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("kitchen", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	path, err := reads.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	if path.State != status.Success {
		t.Fatalf("run state: got %s, want success", path.State)
	}
	want := []struct {
		name string
		kind string
	}{
		{"base", reads.KindStep},
		{"DBOS.sleep", reads.KindWait},
		{"pipeline", reads.KindChild},
		{"DBOS.getResult", reads.KindWait},
		{"side", reads.KindStep},
		{"DBOS.select", reads.KindWait},
		{"spin", reads.KindStep},
		{"spin", reads.KindStep},
	}
	if len(path.Steps) != len(want) {
		t.Fatalf("rows: got %d (%v), want %d", len(path.Steps), stepNames(path.Steps), len(want))
	}
	for i, s := range path.Steps {
		if s.Name != want[i].name || s.Kind != want[i].kind {
			t.Errorf("row %d: got %s (%s), want %s (%s)", i+1, s.Name, s.Kind, want[i].name, want[i].kind)
		}
	}
	// The child row: the child id plus the child workflow's name and
	// badge state (fetched from the child rows) — the row itself is the
	// dispatch and never completes (the wait row does).
	child := path.Steps[2]
	if child.ChildID == "" {
		t.Fatal("child row without a child id")
	}
	if child.ChildName != "pipeline" || child.ChildState != status.Success {
		t.Fatalf("child: got %s (%s), want pipeline (success)", child.ChildName, child.ChildState)
	}
	if child.StartedAt.IsZero() {
		t.Fatal("child row without a started_at")
	}
	if !child.CompletedAt.IsZero() {
		t.Fatal("child row with a completed_at (the wait row is the one that completes)")
	}
}

// A run the core does not know errors with ErrUnknownExecution (the web
// maps it to a 404).
func TestStepsOfUnknownExecution(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := reads.Steps("00000000-0000-0000-0000-000000000000"); !errors.Is(err, reads.ErrUnknownExecution) {
		t.Fatalf("Steps unknown: got %v, want ErrUnknownExecution", err)
	}
}

func stepNames(steps []reads.Step) []string {
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Name)
	}
	return names
}
