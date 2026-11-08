package actions_test

import (
	"errors"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testworkflows"
)

// The resume re-enqueues a failed run by id — the same stored input,
// restarting from the last completed step.

// Resume of an id that is not known by the core errors with
// ErrUnknownExecution (the web maps it to a 404).
func TestResumeUnknownExecution(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := actions.Resume("00000000-0000-0000-0000-000000000000"); !errors.Is(err, reads.ErrUnknownExecution) {
		t.Fatalf("Resume unknown: got %v, want ErrUnknownExecution", err)
	}
}

// Resume of a run that is not failed (here: success) errors with
// ErrNotResumable (the web maps it to a 400).
func TestResumeOfNotFailedRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	if _, err := actions.Resume(ex.ID); !errors.Is(err, actions.ErrNotResumable) {
		t.Fatalf("Resume: got %v, want ErrNotResumable", err)
	}
}

// Resume of a run that is queued errors with ErrNotResumable (the web
// maps it to a 400): only failed runs are resumable. The run sits on a
// queue no executor worker polls (worker concurrency 0), so the queued
// state is deterministic — the executor never claims it.
func TestResumeOfQueuedRun(t *testing.T) {
	testcore.NeedCore(t)
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	st, err := reads.Status(h.GetWorkflowID())
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st != status.Queued {
		t.Fatalf("state: got %s, want queued", st)
	}
	if _, err := actions.Resume(h.GetWorkflowID()); !errors.Is(err, actions.ErrNotResumable) {
		t.Fatalf("Resume of queued: got %v, want ErrNotResumable", err)
	}
}

// Resume of a run that is running errors with ErrNotResumable (the web
// maps it to a 400): only failed runs are resumable. The run (the
// block-test) sits in its block step, and the rejected resume leaves the
// row untouched (still running) — the cancel unblocks the step.
func TestResumeOfRunningRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("block-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	if _, err := actions.Resume(ex.ID); !errors.Is(err, actions.ErrNotResumable) {
		t.Fatalf("Resume of running: got %v, want ErrNotResumable", err)
	}
	// The running run is untouched: the row is still running.
	if st, err := reads.Status(ex.ID); err != nil || st != status.Running {
		t.Fatalf("state after the rejected resume: got %s, want running", st)
	}
	// The cancel unblocks the block step (the suite settles).
	if _, err := actions.Cancel(ex.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
}

// The resume of a cancelled run: the guard accepts the cancelled row, the
// SDK resume re-enqueues it (the resume SQL re-enqueues CANCELLED rows —
// CANCELLED rows — it resumes after the last completed step), and the ACK
// reads the row back: the same id, now enqueued again. The frozen queue
// keeps the run queued until the cancel; the resume re-queues on
// boot.QueueName, so the executor runs it (the noinput-test reaches
// success fast).
func TestResumeOfCancelledRun(t *testing.T) {
	testcore.NeedCore(t)
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := h.GetWorkflowID()
	if st, err := reads.Status(id); err != nil || st != status.Queued {
		t.Fatalf("state before cancel: got %s, want queued", st)
	}
	if _, err := actions.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	ack, err := actions.Resume(id)
	if err != nil {
		t.Fatalf("Resume of cancelled: %v", err)
	}
	if ack.ID != id || ack.State != status.Queued {
		t.Fatalf("ACK: got %s/%s, want the same id enqueued", ack.ID, ack.State)
	}
	// The resume re-queues on boot.QueueName: the executor runs it.
	if _, err := testcore.PollState(t, id, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
}

// The resume of a failed (retries-exhausted) run: the guard lets the error
// state through and the core rewinds the run — the
// SDK deletes the step rows from the first failed row onward (the cut),
// keeps the stored input, resets recovery_attempts, and re-enqueues the
// same id on the same queue. The flaky test workflow provides the
// deterministic failed run; with FlakyFail off, the resumed run reaches
// success.
func TestResumeOfFailedRun(t *testing.T) {
	testcore.NeedCore(t)
	t.Cleanup(func() { testworkflows.FlakyFail = false })
	testworkflows.FlakyFail = true
	ex, err := actions.Enqueue("resume-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to failed: %v", err)
	}
	// The flaky step passes now: the resumed run reaches success.
	testworkflows.FlakyFail = false
	ack, err := actions.Resume(ex.ID)
	if err != nil {
		t.Fatalf("Resume of failed run: %v", err)
	}
	if ack.ID != ex.ID || ack.State != status.Queued {
		t.Fatalf("ACK: got %s/%s, want the same id enqueued", ack.ID, ack.State)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	// The cut: the failed row is deleted from operation_outputs — the
	// executed path has exactly one flaky step, now success.
	path, err := reads.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	if len(path.Steps) != 1 || path.Steps[0].Name != "flaky" || path.Steps[0].State != status.Success {
		t.Fatalf("steps after resume: got %+v", path.Steps)
	}
}
