// The failed read of the failed workflows (the failed page): one row per
// workflow whose latest run is error or cancelled, with the error message
// of the error rows (the batched error payload), in the SDK's desc order
// (newest first, no re-sort). Every booted test owns the runs it fires (the
// fresh per-test file starts empty: the rows are the test's own
// contribution), and every fired run is driven to a terminal state by its
// owning test (the error · the cancel · the ok re-fire).
package board_test

import (
	"errors"
	"testing"
	"time"

	"flowdeck/core/actions"
	"flowdeck/core/board"
	"flowdeck/core/boot"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
)

// The guard (the mock suite): not up returns ErrNotLaunched (the env
// context is up in the env suite — the read is exercised there).
func TestFailedNotLaunchedIsError(t *testing.T) {
	if boot.Ctx() != nil {
		t.Skip("the env context is up — the guard is exercised only on the mock suite")
	}
	if _, err := board.Failed(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("err: %v, want boot.ErrNotLaunched", err)
	}
}

// The fresh per-test file starts empty: no failed run is zero rows.
func TestFailedEmpty(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	quiesce(t)
	runs, err := board.Failed()
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if len(runs) != 0 {
		t.Fatalf("rows: got %d, want 0", len(runs))
	}
}

// The error row: the failing-test fire (the step always fails — the row is
// ERROR after the retries run out) is one row with the step error message
// (F9 · the batched error payload), the enqueued id, and the RFC3339
// created_at.
func TestFailedErrorRow(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	quiesce(t)
	ex, err := actions.Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to error: %v", err)
	}
	runs, err := board.Failed()
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("rows: got %d, want 1", len(runs))
	}
	r := runs[0]
	if r.Name != "failing-test" || r.ID != ex.ID || r.State != "error" {
		t.Errorf("row: got %+v, want {Name: failing-test · ID: %s · State: error}", r, ex.ID)
	}
	if r.Message != "boom: step failed" {
		t.Errorf("message: got %q, want %q", r.Message, "boom: step failed")
	}
	if _, err := time.Parse(time.RFC3339, r.CreatedAt); err != nil {
		t.Errorf("CreatedAt %q is not RFC3339: %v", r.CreatedAt, err)
	}
}

// The cancelled row: the live-test "ok" fire on live-frozen (the frozen
// queue — the executor never claims it) is cancelled in place (the row is
// CANCELLED at the SDK call) and rows with the empty message (no trace — the
// execution page has it); the rows keep the desc order (the newer cancelled
// row is before the older error row · F9).
func TestFailedCancelledRow(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	quiesce(t)
	// The older error row (the batched error payload's input).
	er, err := actions.Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, er.ID, status.Error); err != nil {
		t.Fatalf("poll to error: %v", err)
	}
	// The newer cancelled row.
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	ca := h.GetWorkflowID()
	if _, err := testcore.PollState(t, ca, status.Queued); err != nil {
		t.Fatalf("poll to queued: %v", err)
	}
	if _, err := actions.Cancel(ca); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := testcore.PollState(t, ca, status.Cancelled); err != nil {
		t.Fatalf("poll to cancelled: %v", err)
	}
	runs, err := board.Failed()
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if len(runs) != 2 {
		t.Fatalf("rows: got %d, want 2 (the cancelled · the error)", len(runs))
	}
	// The desc order (newest first · F9): the newer cancelled row is the
	// first row, and it carries no message (no trace).
	got := runs[0]
	if got.Name != "live-test" || got.ID != ca || got.State != "cancelled" || got.Message != "" {
		t.Errorf("first row: got %+v, want the live-test cancelled row (empty message)", got)
	}
	got = runs[1]
	if got.Name != "failing-test" || got.ID != er.ID || got.State != "error" || got.Message != "boom: step failed" {
		t.Errorf("second row: got %+v, want the failing-test error row", got)
	}
}

// The latest run wins: a success after the cancelled row removes the
// workflow's row (the failing-test row remains).
func TestFailedLatestRunWins(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	quiesce(t)
	// The two failed rows (the error · the cancelled).
	er, err := actions.Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, er.ID, status.Error); err != nil {
		t.Fatalf("poll to error: %v", err)
	}
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	ca := h.GetWorkflowID()
	if _, err := testcore.PollState(t, ca, status.Queued); err != nil {
		t.Fatalf("poll to queued: %v", err)
	}
	if _, err := actions.Cancel(ca); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := testcore.PollState(t, ca, status.Cancelled); err != nil {
		t.Fatalf("poll to cancelled: %v", err)
	}
	runs, err := board.Failed()
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("rows: got %d, want 2 (before the latest success)", len(runs))
	}
	// The latest success removes the workflow's row.
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	runs, err = board.Failed()
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if len(runs) != 1 {
		t.Errorf("rows: got %d, want 1 (the latest success removed the live-test row)", len(runs))
	}
	if len(runs) == 1 {
		r := runs[0]
		if r.Name != "failing-test" || r.ID != er.ID || r.State != "error" || r.Message != "boom: step failed" {
			t.Errorf("row: got %+v, want the failing-test error row", r)
		}
	}
}
