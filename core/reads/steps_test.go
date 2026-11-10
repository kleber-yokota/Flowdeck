package reads_test

import (
	"errors"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
)

// The executed path of a child-test run: the run's state (the poll
// control) and the rows in order (returned step_id asc), each
// mapped onto its kind — the step row (base), the child row (inner-test —
// the RunWorkflow dispatch: the row's name is the child workflow's name and
// it never completes) and the wait row (DBOS.getResult — the SDK-internal
// name; it completes when the child does).
func TestStepsOfChildDispatchingRun(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("child-test", "FlowDeck")
	if err != nil {
		t.Errorf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Errorf("poll to success: %v", err)
	}
	path, err := reads.Steps(ex.ID)
	if err != nil {
		t.Errorf("Steps: %v", err)
	}
	if path.State != status.Success {
		t.Errorf("run state: got %s, want success", path.State)
	}
	want := []struct {
		name string
		kind string
	}{
		{"base", reads.KindStep},
		{"inner-test", reads.KindChild},
		{"DBOS.getResult", reads.KindWait},
	}
	if len(path.Steps) != len(want) {
		t.Errorf("rows: got %d (%v), want %d", len(path.Steps), stepNames(path.Steps), len(want))
	}
	for i, s := range path.Steps {
		if s.Name != want[i].name || s.Kind != want[i].kind {
			t.Errorf("row %d: got %s (%s), want %s (%s)", i+1, s.Name, s.Kind, want[i].name, want[i].kind)
		}
	}
	// The child row: the child id plus the child workflow's name and
	// badge state (fetched from the child rows) — the row itself is the
	// dispatch and never completes (the wait row does).
	child := path.Steps[1]
	if child.ChildID == "" {
		t.Error("child row without a child id")
	}
	if child.ChildName != "inner-test" || child.ChildState != status.Success {
		t.Errorf("child: got %s (%s), want inner-test (success)", child.ChildName, child.ChildState)
	}
	if child.StartedAt.IsZero() {
		t.Error("child row without a started_at")
	}
	if !child.CompletedAt.IsZero() {
		t.Error("child row with a completed_at (the wait row is the one that completes)")
	}
}

// A run the core does not know errors with ErrUnknownExecution (the web
// maps it to a 404).
func TestStepsOfUnknownExecution(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	if _, err := reads.Steps("00000000-0000-0000-0000-000000000000"); !errors.Is(err, reads.ErrUnknownExecution) {
		t.Errorf("Steps unknown: got %v, want ErrUnknownExecution", err)
	}
}

func stepNames(steps []reads.Step) []string {
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Name)
	}
	return names
}

// The step-detail read: one step row's stored output (the JSON text, the
// echo step's "fetch") and — for a failed step — its error (the expand
// fragment's read, GET /executions/{id}/steps/{step}).
func TestStepDetailOfCompletedStep(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("linear-test", "FlowDeck")
	if err != nil {
		t.Errorf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Errorf("poll to success: %v", err)
	}
	d, err := reads.StepDetailOf(ex.ID, "fetch")
	if err != nil {
		t.Errorf("StepDetailOf: %v", err)
	}
	if d.Output != "\"fetch\"" {
		t.Errorf("Output: got %q, want %q", d.Output, "\"fetch\"")
	}
	if d.Error != "" {
		t.Errorf("Error: got %q, want empty", d.Error)
	}
}

// A failed step carries its error and no output (the fixture's pinned
// error text, fact.md row 7).
func TestStepDetailOfFailedStep(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("failing-test", nil)
	if err != nil {
		t.Errorf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Errorf("poll to error: %v", err)
	}
	d, err := reads.StepDetailOf(ex.ID, "boom-step")
	if err != nil {
		t.Errorf("StepDetailOf: %v", err)
	}
	if d.Error != "boom: step failed" {
		t.Errorf("Error: got %q, want %q", d.Error, "boom: step failed")
	}
	if d.Output != "" {
		t.Errorf("Output: got %q, want empty", d.Output)
	}
}

// A run the core does not know errors with ErrUnknownExecution (the web
// maps it to a 404) — the unknown id of the step-detail read.
func TestStepDetailUnknownExecution(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	if _, err := reads.StepDetailOf("00000000-0000-0000-0000-000000000000", "fetch"); !errors.Is(err, reads.ErrUnknownExecution) {
		t.Errorf("StepDetailOf unknown: got %v, want ErrUnknownExecution", err)
	}
}

// A step name the run never executed is a wrapped error (no sentinel —
// the web's Fail maps it to the 500 default).
func TestStepDetailUnknownStep(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("linear-test", "FlowDeck")
	if err != nil {
		t.Errorf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Errorf("poll to success: %v", err)
	}
	if _, err := reads.StepDetailOf(ex.ID, "nope"); err == nil {
		t.Error("StepDetailOf unknown step: want an error, got none")
	}
}
