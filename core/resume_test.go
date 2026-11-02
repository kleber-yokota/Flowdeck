package core_test

import (
	"errors"
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// Phase 5, RESUME: the resume re-enqueues a failed run by id — the same
// stored input, restarting from the last completed step.

// Resume of an id that is not known by the core errors with
// ErrUnknownExecution (the web maps it to a 404).
func TestResumeUnknownExecution(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := core.Resume("00000000-0000-0000-0000-000000000000"); !errors.Is(err, reads.ErrUnknownExecution) {
		t.Fatalf("Resume unknown: got %v, want ErrUnknownExecution", err)
	}
}

// Resume of a run that is not failed (here: success) errors with
// ErrNotResumable (the web maps it to a 400).
func TestResumeOfNotFailedRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := core.Enqueue("hello-world", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	if _, err := core.Resume(ex.ID); !errors.Is(err, core.ErrNotResumable) {
		t.Fatalf("Resume: got %v, want ErrNotResumable", err)
	}
}

// Resume of a run that is queued errors with ErrNotResumable (the web
// maps it to a 400): only failed runs are resumable. The run sits on a
// queue no executor worker polls (worker concurrency 0), so the queued
// state is deterministic — the executor never claims it.
func TestResumeOfQueuedRun(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := dbos.RegisterQueue(boot.Ctx(), "frozen", dbos.WithWorkerConcurrency(0)); err != nil {
		t.Fatalf("RegisterQueue: %v", err)
	}
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "pause", nil)
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
	if _, err := core.Resume(h.GetWorkflowID()); !errors.Is(err, core.ErrNotResumable) {
		t.Fatalf("Resume of queued: got %v, want ErrNotResumable", err)
	}
}

// Resume of a run that is running errors with ErrNotResumable (the web
// maps it to a 400): only failed runs are resumable. The run (the pause)
// then finishes normally.
func TestResumeOfRunningRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := core.Enqueue("pause", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	if _, err := core.Resume(ex.ID); !errors.Is(err, core.ErrNotResumable) {
		t.Fatalf("Resume of running: got %v, want ErrNotResumable", err)
	}
	// The running run is untouched: it finishes with success.
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
}

// The resume of a cancelled run errors with ErrNotResumable (the web maps
// it to a 400): only failed runs are resumable.
func TestResumeOfCancelledRun(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := dbos.RegisterQueue(boot.Ctx(), "frozen", dbos.WithWorkerConcurrency(0)); err != nil {
		t.Fatalf("RegisterQueue: %v", err)
	}
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "pause", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := h.GetWorkflowID()
	if _, err := core.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := core.Resume(id); !errors.Is(err, core.ErrNotResumable) {
		t.Fatalf("Resume of cancelled: got %v, want ErrNotResumable", err)
	}
}

// The resume of a failed (retries-exhausted) run is not resumable today:
// the guard lets the error state through, but the SDK's ResumeWorkflow is a
// no-op on ERROR rows (bug.md) — the ACK read-back is still error. The
// flaky test workflow provides the deterministic failed run with a known
// stored input.
func TestResumeOfFailedRun(t *testing.T) {
	testcore.NeedCore(t)
	t.Cleanup(func() { testcore.FlakyFail = false })
	testcore.FlakyFail = true
	ex, err := core.Enqueue("resume-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	st, err := testcore.PollState(t, ex.ID, status.Error)
	if err != nil {
		t.Fatalf("poll to failed: %v (state %s)", err, st)
	}
	if st != status.Error {
		t.Fatalf("failing run state: got %s, want error", st)
	}
	if _, err := core.Resume(ex.ID); !errors.Is(err, core.ErrNotResumable) {
		t.Fatalf("Resume of failed run: got %v, want ErrNotResumable", err)
	}
}
