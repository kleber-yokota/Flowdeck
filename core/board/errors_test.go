package board_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"flowdeck/core/actions"
	"flowdeck/core/board"
	"flowdeck/core/boot"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
)

// ensureUnbooted makes sure the singleton is down (the previous test's
// shutdown leaves it down — the guard is defensive, the tests run in
// order).
func ensureUnbooted(t *testing.T) {
	t.Helper()
	if boot.Ctx() != nil {
		boot.Shutdown(time.Second)
	}
}

// A not-launched core: every live-board read returns ErrNotLaunched
// (the web maps the 503).
func TestLiveBoardReadsOfAnUnbootedCore(t *testing.T) {
	ensureUnbooted(t)
	if _, err := board.LiveCounts(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("LiveCounts unbooted: got %v, want ErrNotLaunched", err)
	}
	if _, err := board.Living(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Living unbooted: got %v, want ErrNotLaunched", err)
	}
	if _, err := board.Queued(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Queued unbooted: got %v, want ErrNotLaunched", err)
	}
}

// The live board reads the status rows: with the status table dropped
// (a second connection — the real DB, no mock), the list query fails and
// the read returns the wrapped error.
func TestLiveBoardReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	testcore.DropTable(t, dbfile, "workflow_status")

	if _, err := board.LiveCounts(); err == nil || !strings.Contains(err.Error(), "core: live") {
		t.Fatalf("LiveCounts read failure: got %v, want the wrapped list error", err)
	}
	if _, err := board.Living(); err == nil || !strings.Contains(err.Error(), "core: living") {
		t.Fatalf("Living read failure: got %v, want the wrapped list error", err)
	}
	if _, err := board.Queued(); err == nil || !strings.Contains(err.Error(), "core: queued") {
		t.Fatalf("Queued read failure: got %v, want the wrapped list error", err)
	}
	if _, err := board.Failed(); err == nil || !strings.Contains(err.Error(), "core: failed") {
		t.Fatalf("Failed read failure: got %v, want the wrapped list error", err)
	}
}

// The failed read maps every status row: a row the SDK cannot produce
// (a second connection sets status BOGUS) is rejected, not guessed.
func TestFailedOfAnUnmappedState(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", "")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll: %v", err)
	}
	testcore.SetUnmappedState(t, dbfile, ex.ID)
	if _, err := board.Failed(); err == nil || !strings.Contains(err.Error(), "unmapped workflow state") {
		t.Fatalf("Failed of an unmapped state: got %v, want ErrUnmappedState", err)
	}
}

// The live counts map every status row: an unmapped row is rejected
// (the dashboard cannot show a bucket of an unknown state).
func TestLiveCountsOfAnUnmappedState(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", "")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll: %v", err)
	}
	testcore.SetUnmappedState(t, dbfile, ex.ID)
	if _, err := board.LiveCounts(); err == nil || !strings.Contains(err.Error(), "unmapped workflow state") {
		t.Fatalf("LiveCounts of an unmapped state: got %v, want ErrUnmappedState", err)
	}
}

// The living and queued lists map every status row: an unmapped row is
// rejected (a live list cannot render an unknown state).
func TestLivingAndQueuedOfAnUnmappedState(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", "")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll: %v", err)
	}
	testcore.SetUnmappedState(t, dbfile, ex.ID)
	if _, err := board.Living(); err == nil || !strings.Contains(err.Error(), "unmapped workflow state") {
		t.Fatalf("Living of an unmapped state: got %v, want ErrUnmappedState", err)
	}
	if _, err := board.Queued(); err == nil || !strings.Contains(err.Error(), "unmapped workflow state") {
		t.Fatalf("Queued of an unmapped state: got %v, want ErrUnmappedState", err)
	}
}

// The failed read's error payload is the batched error query: with the
// output table dropped (a second connection), the batched query fails
// and the read returns the wrapped error.
func TestFailedErrorMessagesReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("failing-test", 1)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll: %v", err)
	}
	testcore.DropTable(t, dbfile, "workflow_output")
	if _, err := board.Failed(); err == nil || !strings.Contains(err.Error(), "core: failed") {
		t.Fatalf("Failed error-messages failure: got %v, want the wrapped list error", err)
	}
}
