package board_test

import (
	"errors"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/board"
	"flowdeck/core/boot"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
)

// The live read of the running runs (the runnings page): one row per
// run running right now — the rows are runs, not workflows: the same
// workflow running twice is two rows (no dedup by workflow name). Every
// env test owns the test-only workflow "live-test", starts from the
// quiesced baseline, and ends with live-test's latest run in SUCCESS
// (start state = end state: the live-test contribution is zero).

// Two block fires of live-test are two rows of the same name (no dedup
// by workflow name); the terminal runs are out of the list again after
// the cancel + the latest success (the suite contract).
//
// The row is PENDING until the cancel; the strict row count is sound
// (F3): the main queue is registered with no worker limit
// (core/boot/lifecycle.go — RegisterQueue without WithWorkerConcurrency),
// so the executor claims every fire and both runs are PENDING (running)
// at once; the cancels are the release (the block step unblocks at the
// 100ms poll — no leftover tail).
func TestLivingNoDedupByWorkflow(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	quiesce(t)
	fireIDs := make([]string, 2)
	for i := range fireIDs {
		ex, err := actions.Enqueue("live-test", "block")
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		fireIDs[i] = ex.ID
	}
	for _, id := range fireIDs {
		if _, err := testcore.PollState(t, id, status.Running); err != nil {
			t.Fatalf("poll to running: %v", err)
		}
	}
	runs, err := board.Living()
	if err != nil {
		t.Fatalf("Living: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("rows: got %d, want 2 (no dedup by workflow name)", len(runs))
	}
	for _, r := range runs {
		if r.Name != "live-test" {
			t.Fatalf("row name: got %q, want live-test", r.Name)
		}
		if r.State != "running" {
			t.Fatalf("row state: got %q, want running", r.State)
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
	absentFromLiving(t, fireIDs[0])
	absentFromLiving(t, fireIDs[1])
}

// A success run and an error run are out of the list (only the running
// runs are rows); the closing success is the suite contract.
func TestLivingTerminalRunsAbsent(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	quiesce(t)
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	absentFromLiving(t, ok.ID)
	er, err := actions.Enqueue("live-test", "err")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, er.ID, status.Error); err != nil {
		t.Fatalf("poll to error: %v", err)
	}
	absentFromLiving(t, er.ID)
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
func TestLivingNotLaunchedIsError(t *testing.T) {
	if boot.Ctx() != nil {
		t.Skip("the env context is up — the guard is exercised only on the mock suite")
	}
	if _, err := board.Living(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("err: %v, want boot.ErrNotLaunched", err)
	}
}

// absentFromLiving fails if id is a row of the running runs.
func absentFromLiving(t *testing.T, id string) {
	t.Helper()
	runs, err := board.Living()
	if err != nil {
		t.Fatalf("Living: %v", err)
	}
	for _, r := range runs {
		if r.ID == id {
			t.Fatalf("the terminal run %s is a row of the running runs", id)
		}
	}
}
