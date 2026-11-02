package core_test

import (
	"errors"
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/status"
)

// The cancel of a live run by id (the cancel sequence): the guard lets
// only queued/running through, the SDK flips the row to CANCELLED and takes
// it out of the queue, and the ACK reads the row back (the same id, now
// cancelled — the row flips at the SDK call, no wait for the running step).

// The cancel of an id that is not known by the core errors with
// ErrUnknownExecution (the web maps it to a 404).
func TestCancelUnknownExecution(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := core.Cancel("00000000-0000-0000-0000-000000000000"); !errors.Is(err, core.ErrUnknownExecution) {
		t.Fatalf("Cancel unknown: got %v, want ErrUnknownExecution", err)
	}
}

// The cancel of a run that is not live (here: success) errors with
// ErrNotCancellable (the web maps it to a 400).
func TestCancelOfNotLiveRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := core.Enqueue("hello-world", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	if _, err := core.Cancel(ex.ID); !errors.Is(err, core.ErrNotCancellable) {
		t.Fatalf("Cancel: got %v, want ErrNotCancellable", err)
	}
}

// The cancel of a queued run: the run sits on a queue no executor worker
// polls (worker concurrency 0), so the queued state is deterministic — the
// executor never claims it. The ACK is the same id, now cancelled.
func TestCancelQueuedRun(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := dbos.RegisterQueue(boot.Ctx(), "frozen", dbos.WithWorkerConcurrency(0)); err != nil {
		t.Fatalf("RegisterQueue: %v", err)
	}
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "pause", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := h.GetWorkflowID()
	st, err := core.Status(id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st != status.Queued {
		t.Fatalf("state: got %s, want queued", st)
	}
	ack, err := core.Cancel(id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ack.ID != id {
		t.Fatalf("ACK id: got %s, want %s", ack.ID, id)
	}
	if ack.State != status.Cancelled {
		t.Fatalf("ACK state: got %s, want cancelled", ack.State)
	}
	if ack.CreatedAt.IsZero() {
		t.Fatal("ACK without a created_at")
	}
}

// The cancel of a running run: the row flips at the SDK call (no wait for
// the step — the running step completes in-process but the row stays
// CANCELLED). The ACK is the same id, now cancelled.
func TestCancelRunningRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := core.Enqueue("pause", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	st, err := testcore.PollState(t, ex.ID, status.Running)
	if err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	if st != status.Running {
		t.Fatalf("state: got %s, want running", st)
	}
	ack, err := core.Cancel(ex.ID)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if ack.ID != ex.ID {
		t.Fatalf("ACK id: got %s, want %s", ack.ID, ex.ID)
	}
	if ack.State != status.Cancelled {
		t.Fatalf("ACK state: got %s, want cancelled", ack.State)
	}
}
