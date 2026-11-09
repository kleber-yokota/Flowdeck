package reads_test

import (
	"errors"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
)

// The live read of the queued runs (the queued page): one row per queued
// run (the ENQUEUED ∪ DELAYED bucket) — the rows are runs, not
// workflows: the same workflow queued twice is two rows (no dedup by
// workflow name). Every env test owns the test-only workflow
// "live-test", starts from the quiesced baseline, and ends with
// live-test's latest run in SUCCESS (start state = end state: the
// live-test contribution is zero).
//
// Two "ok" fires on live-frozen (the frozen queue — worker concurrency
// 0) are two rows of the same name: the executor never claims them, so
// the Status read is queued immediately (no poll to a claim, no timing
// window). The terminal runs are out of the list after the cancel + the
// latest success (the suite contract).

func TestQueuedNoDedupByWorkflow(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	quiesce(t)
	fireIDs := make([]string, 2)
	for i := range fireIDs {
		h, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		fireIDs[i] = h.GetWorkflowID()
	}
	for _, id := range fireIDs {
		if _, err := testcore.PollState(t, id, status.Queued); err != nil {
			t.Fatalf("poll to queued: %v", err)
		}
	}
	runs, err := reads.Queued()
	if err != nil {
		t.Fatalf("Queued: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("rows: got %d, want 2 (no dedup by workflow name)", len(runs))
	}
	for _, r := range runs {
		if r.Name != "live-test" {
			t.Fatalf("row name: got %q, want live-test", r.Name)
		}
		if r.State != "queued" {
			t.Fatalf("row state: got %q, want queued", r.State)
		}
	}
	have := map[string]bool{runs[0].ID: true, runs[1].ID: true}
	for _, id := range fireIDs {
		if !have[id] {
			t.Fatalf("the rows are not the two fires: got %v, want %v",
				[]string{runs[0].ID, runs[1].ID}, fireIDs)
		}
	}
	// The suite contract: the two cancel + a latest success, and the two
	// terminal ids are out of the list again.
	for _, id := range fireIDs {
		if _, err := actions.Cancel(id); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	}
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	absentFromQueued(t, fireIDs[0])
	absentFromQueued(t, fireIDs[1])
}

// A success run and a cancelled run are out of the list (only the queued
// runs are rows); the closing success is the suite contract.
func TestQueuedTerminalRunsAbsent(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	quiesce(t)
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	absentFromQueued(t, ok.ID)
	// The cancelled run: a block fire sits in its block step (the
	// deterministic running state — no timing window), and the cancel
	// flips the row to CANCELLED at the SDK call.
	block, err := actions.Enqueue("live-test", "block")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, block.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	if _, err := actions.Cancel(block.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	absentFromQueued(t, block.ID)
	// The suite contract: live-test's latest run is SUCCESS.
	last, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, last.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
}

// The guard (the mock suite): not up returns ErrNotLaunched (the env
// context is up in the env suite — the read is exercised there).
func TestQueuedNotLaunchedIsError(t *testing.T) {
	if boot.Ctx() != nil {
		t.Skip("the env context is up — the guard is exercised only on the mock suite")
	}
	if _, err := reads.Queued(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("err: %v, want boot.ErrNotLaunched", err)
	}
}

// absentFromQueued fails if id is a row of the queued runs.
func absentFromQueued(t *testing.T, id string) {
	t.Helper()
	runs, err := reads.Queued()
	if err != nil {
		t.Fatalf("Queued: %v", err)
	}
	for _, r := range runs {
		if r.ID == id {
			t.Fatalf("the terminal run %s is a row of the queued runs", id)
		}
	}
}
