package core_test

import (
	"errors"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/status"
)

// TestPortLivingNotLaunchedIsError (the guard): the not-launched port
// returns ErrNotLaunched from the Living field (like every other read).
func TestPortLivingNotLaunchedIsError(t *testing.T) {
	if boot.Ctx() != nil {
		t.Skip("the env context is up — the guard is exercised only on the mock suite")
	}
	p := core.NewPort()
	if _, err := p.Living(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Living: %v, want boot.ErrNotLaunched", err)
	}
}

// TestPortLivingRowsTheRunningRun (the contract, no stubs): the Port's
// Living field rows the running run — the block fire is one row (name ·
// id · running · the RFC 3339 created_at); the suite contract settles it
// (the inline cancel + the ok fire, fact F2).
func TestPortLivingRowsTheRunningRun(t *testing.T) {
	testcore.BootTest(t, "testworkflows")
	fired, err := actions.Enqueue("live-test", "block")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, fired.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}

	p := core.NewPort()
	runs, err := p.Living()
	if err != nil {
		t.Fatalf("Living: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("Living rows: got %d (%+v), want the one running run", len(runs), runs)
	}
	row := runs[0]
	if row.ID != fired.ID || row.Name != "live-test" || row.State != string(status.Running) {
		t.Fatalf("the fired run's row: got %+v, want id %s · name live-test · state running", row, fired.ID)
	}
	if _, err := time.Parse(time.RFC3339, row.CreatedAt); err != nil {
		t.Fatalf("created_at: %q is not RFC 3339: %v", row.CreatedAt, err)
	}

	// Settle (fact F2): the block fire is cancelled inside the test, then
	// an ok fire runs to success — the suite leaves no live-test residue.
	if _, err := actions.Cancel(fired.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	ok, err := actions.Enqueue("live-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue (ok): %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
}
