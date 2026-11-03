package reads_test

import (
	"errors"
	"flowdeck/core/actions"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"strings"
	"testing"
)

// A run that fails in a step reaches the failed state, and Detail shows the
// error and the failed step (phase 3, DETAIL: on failure: error + the failed
// step).
// Phase 5, DETAIL: the detail shows the stored input values — the values
// the run used (the input is written once at enqueue, immutable per
// workflow_id).
func TestDetailShowsInputValues(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("hello-world", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	d, err := reads.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(d.Inputs) != 1 || d.Inputs[0].Name != "msg" || d.Inputs[0].Value != "FlowDeck" {
		t.Fatalf("inputs: got %+v, want [{msg FlowDeck}]", d.Inputs)
	}
}

// A struct input shows one value per field (the map the core enqueued,
// stored by DBOS).
func TestDetailShowsStructInputValues(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("schedule", map[string]any{"Name": "Ada", "StartDate": "2026-01-01"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	d, err := reads.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	want := []reads.InputValue{{Name: "Name", Value: "Ada"}, {Name: "StartDate", Value: "2026-01-01"}}
	if len(d.Inputs) != len(want) {
		t.Fatalf("inputs: got %+v, want %v", d.Inputs, want)
	}
	for i, in := range want {
		if d.Inputs[i] != in {
			t.Fatalf("input %d: got %+v, want %+v", i, d.Inputs[i], in)
		}
	}
}

func TestFailingRunDetailHasErrorAndFailedStep(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	st, err := testcore.PollState(t, ex.ID, status.Error)
	if err != nil {
		t.Fatalf("poll to error: %v", err)
	}
	if st != status.Error {
		t.Fatalf("failing run state: got %s, want error", st)
	}
	d, err := reads.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if !strings.Contains(d.Error, "boom: step failed") {
		t.Fatalf("Detail error: got %q", d.Error)
	}
	if d.FailedStep != "boom-step" {
		t.Fatalf("Detail failed step: got %q, want boom-step", d.FailedStep)
	}
	if d.Attempts < 1 {
		t.Fatalf("Detail attempts: %d, want >= 1", d.Attempts)
	}
}

// The children of a child-dispatching run (task 03): Detail lists one
// entry per child workflow (the child id, the child's name, the child's
// badge state) — the read model the detail page's child list renders.
func TestDetailChildrenOfChildDispatchingRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("kitchen", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	d, err := reads.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(d.Children) != 1 {
		t.Fatalf("children: got %v, want exactly one", d.Children)
	}
	c := d.Children[0]
	if c.ID == "" {
		t.Fatal("child without an id")
	}
	if c.Name != "pipeline" || c.State != status.Success {
		t.Fatalf("child: got %s (%s), want pipeline (success)", c.Name, c.State)
	}
}

// A run the core does not know errors with ErrUnknownExecution (the web
// maps it to a 404).
func TestDetailOfUnknownExecution(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := reads.Detail("00000000-0000-0000-0000-000000000000"); !errors.Is(err, reads.ErrUnknownExecution) {
		t.Fatalf("Detail unknown: got %v, want ErrUnknownExecution", err)
	}
}
