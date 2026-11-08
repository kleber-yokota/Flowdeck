package actions_test

import (
	"errors"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/registry"
	"flowdeck/core/status"
)

// Enqueue (the dispatch) returns the new execution (id + state +
// created_at) — the dispatch ACK the web renders as the "queued" row.
func TestEnqueueReturnsExecution(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("live-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if ex.ID == "" || ex.ID == "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("ACK without an execution id: %+v", ex)
	}
	if ex.State != status.Queued && ex.State != status.Running {
		t.Fatalf("ACK state: got %s, want queued/running", ex.State)
	}
	if ex.CreatedAt.IsZero() {
		t.Fatal("ACK without a created_at")
	}
}

// Enqueue of a name that is not in the registry errors with
// ErrUnknownWorkflow (the web maps it to a 400).
func TestEnqueueUnknownWorkflow(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	if _, err := actions.Enqueue("no-such-workflow", nil); !errors.Is(err, registry.ErrUnknownWorkflow) {
		t.Fatalf("Enqueue unknown: got %v, want ErrUnknownWorkflow", err)
	}
}

// The dispatch enqueues the clean input (the web sanitized the raw form
// values against the spec) — the run takes the value (the fork-test
// workflow branches on it: base + left, no right).
func TestEnqueueWithValuesTakesBranch(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("fork-test", "left")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	for _, name := range testcore.MustStepNames(t, ex) {
		if name == "right" {
			t.Fatalf("steps: right ran, want the left branch: %v", testcore.MustStepNames(t, ex))
		}
	}
}
