package core_test

import (
	"testing"

	"flowdeck/core"
	"flowdeck/core/status"
)

// The rows of the executed path of the run with every row kind (phase 4,
// STEP 3): the step rows (base, side — the plain steps; spin — one row per
// iteration), the wait rows (DBOS.sleep, DBOS.getResult, DBOS.select — the
// SDK-internal names) and the child row (pipeline — the RunWorkflow
// dispatch: the row's name is the child workflow's name and it never
// completes; the wait row does).

// The steps of a kitchen run: the eight rows, in order (Postgres returns
// them step_id asc), each mapped onto its kind.
func TestStepsOfKitchenRun(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("kitchen", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	steps, err := core.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	want := []struct {
		name string
		kind string
	}{
		{"base", core.KindStep},
		{"DBOS.sleep", core.KindWait},
		{"pipeline", core.KindChild},
		{"DBOS.getResult", core.KindWait},
		{"side", core.KindStep},
		{"DBOS.select", core.KindWait},
		{"spin", core.KindStep},
		{"spin", core.KindStep},
	}
	if len(steps) != len(want) {
		t.Fatalf("rows: got %d (%v), want %d", len(steps), stepNames(steps), len(want))
	}
	for i, s := range steps {
		if s.Name != want[i].name || s.Kind != want[i].kind {
			t.Errorf("row %d: got %s (%s), want %s (%s)", i+1, s.Name, s.Kind, want[i].name, want[i].kind)
		}
	}
	// The child row: the child id plus the child workflow's name and
	// badge state (fetched from the child rows) — the row itself is the
	// dispatch and never completes (the wait row does).
	child := steps[2]
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

func stepNames(steps []core.Step) []string {
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Name)
	}
	return names
}
