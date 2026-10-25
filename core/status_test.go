package core

import (
	"errors"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/status"
)

// mapState is the phase 2 contract: every DBOS state lands on one of the
// four UI states, and unknown states are rejected, not guessed.
func TestMapState(t *testing.T) {
	for state, want := range map[dbos.WorkflowStatusType]status.State{
		dbos.WorkflowStatusEnqueued:  status.Queued,
		dbos.WorkflowStatusDelayed:   status.Queued,
		dbos.WorkflowStatusPending:   status.Running,
		dbos.WorkflowStatusSuccess:   status.Success,
		dbos.WorkflowStatusError:     status.Error,
		dbos.WorkflowStatusCancelled: status.Error,
	} {
		got, err := mapState(state)
		if err != nil {
			t.Fatalf("mapState(%s): %v", state, err)
		}
		if got != want {
			t.Errorf("mapState(%s): got %s, want %s", state, got, want)
		}
	}

	if _, err := mapState(dbos.WorkflowStatusType("bogus")); err == nil {
		t.Error("mapState(bogus): expected error, got nil")
	}
}

// Status before Launch must fail cleanly (core not up yet).
func TestStatusWithoutLaunchErrors(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if _, err := Status("00000000-0000-0000-0000-000000000001"); !errors.Is(err, ErrNotLaunched) {
		t.Fatalf("Status before Launch: got %v, want ErrNotLaunched", err)
	}
}

// Status must reject an ID that is not known by the core (the web maps it
// to a 404).
func TestStatusUnknownExecution(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	if _, err := Status("00000000-0000-0000-0000-000000000001"); !errors.Is(err, ErrUnknownExecution) {
		t.Fatalf("Status unknown: got %v, want ErrUnknownExecution", err)
	}
}

// Enqueue -> executor -> Status: the phase 2 loop, asserted through the
// read half (the web polls Status every 2s and swaps the badge).
func TestStatusReachesSuccess(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	ex, err := Enqueue("hello-world", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := ex.ID

	deadline := time.Now().Add(30 * time.Second)
	for {
		state, err := Status(id)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		switch state {
		case status.Queued, status.Running:
			// keep polling
		case status.Success:
			return
		default:
			t.Fatalf("unexpected terminal state: %s", state)
		}
		if time.Now().After(deadline) {
			t.Fatalf("did not reach success in time (last state: %s)", state)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
