package actions_test

import (
	"errors"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// The cancel of a live run by id (the cancel sequence): the guard lets
// only queued/running through, the SDK flips the row to CANCELLED and takes
// it out of the queue, and the ACK reads the row back (the same id, now
// cancelled — the row flips at the SDK call, no wait for the running step).

// The cancel of an id that is not known by the core errors with
// ErrUnknownExecution (the web maps it to a 404).
func TestCancelUnknownExecution(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	if _, err := actions.Cancel("00000000-0000-0000-0000-000000000000"); !errors.Is(err, reads.ErrUnknownExecution) {
		t.Fatalf("Cancel unknown: got %v, want ErrUnknownExecution", err)
	}
}

// The cancel of a run that is not live (here: success) errors with
// ErrNotCancellable (the web maps it to a 400).
func TestCancelOfNotLiveRun(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	if _, err := actions.Cancel(ex.ID); !errors.Is(err, actions.ErrNotCancellable) {
		t.Fatalf("Cancel: got %v, want ErrNotCancellable", err)
	}
}

// The cancel of a queued run: the run sits on a queue no executor worker
// polls (worker concurrency 0), so the queued state is deterministic — the
// executor never claims it. The ACK is the same id, now cancelled.
func TestCancelQueuedRun(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := h.GetWorkflowID()
	st, err := reads.Status(id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st != status.Queued {
		t.Fatalf("state: got %s, want queued", st)
	}
	ack, err := actions.Cancel(id)
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

// The cancel of a running run: the block-test run sits in its block step
// (it blocks until the run is cancelled — the deterministic Running state,
// no timing window). The row flips at the SDK call and the step unblocks
// at the cancel (no leftover tail: the suite settles). The ACK is the same
// id, now cancelled.
func TestCancelRunningRun(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("block-test", nil)
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
	ack, err := actions.Cancel(ex.ID)
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
