package core_test

import (
	"encoding/json"
	"errors"
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
	"flowdeck/core/status"
)

// Phase 5, RESUME: the resume re-enqueues a failed run by id — the same
// stored input, restarting from the last completed step.

// Resume of an id that is not known by the core errors with
// ErrUnknownExecution (the web maps it to a 404).
func TestResumeUnknownExecution(t *testing.T) {
	needCore(t)
	if _, err := core.Resume("00000000-0000-0000-0000-000000000000"); !errors.Is(err, core.ErrUnknownExecution) {
		t.Fatalf("Resume unknown: got %v, want ErrUnknownExecution", err)
	}
}

// Resume of a run that is not failed (here: success) errors with
// ErrNotResumable (the web maps it to a 400).
func TestResumeOfNotFailedRun(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("hello-world", map[string]string{"msg": "FlowDeck"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	if _, err := core.Resume(ex.ID); !errors.Is(err, core.ErrNotResumable) {
		t.Fatalf("Resume: got %v, want ErrNotResumable", err)
	}
}

// The resume flow on a canceled run: the run is canceled mid-step (the
// pause workflow), the resume re-enqueues it by id (ACK: the same id,
// queued again), and the restarted run finishes with the same stored
// input (the output echoes the dispatch value).
func TestResumeRestartsFailedRunWithSameInput(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("pause", map[string]string{"msg": "same-input"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// Wait until the run is active (the step is pausing).
	if _, err := pollState(t, ex.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	// Cancel the run mid-step: the badge state becomes failed (error).
	if err := dbos.CancelWorkflow(core.Ctx(), ex.ID); err != nil {
		t.Fatalf("CancelWorkflow: %v", err)
	}
	if st, err := pollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to failed: %v (state %s)", err, st)
	}

	// The resume: the ACK is the same id, enqueued again.
	ack, err := core.Resume(ex.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if ack.ID != ex.ID {
		t.Fatalf("ACK id: got %s, want %s", ack.ID, ex.ID)
	}
	if ack.State != status.Queued && ack.State != status.Running {
		t.Fatalf("ACK state: got %s, want queued/running", ack.State)
	}
	// The restarted run reaches success.
	if _, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}

	// The same stored input: the restarted run's output echoes the
	// dispatch value (the resume never changes the stored input).
	statuses, err := dbos.ListWorkflows(core.Ctx(),
		dbos.WithFilterWorkflowIDs(ex.ID),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(true),
	)
	if err != nil {
		t.Fatalf("ListWorkflows: %v", err)
	}
	if len(statuses) != 1 {
		t.Fatalf("ListWorkflows: %d rows, want 1", len(statuses))
	}
	// The stored output is the JSON of the workflow's return value.
	var out string
	if err := json.Unmarshal([]byte(statuses[0].Output.(string)), &out); err != nil {
		t.Fatalf("unmarshal output: %v", err)
	}
	if out != "paused: same-input" {
		t.Fatalf("output: got %q, want %q", out, "paused: same-input")
	}
}
