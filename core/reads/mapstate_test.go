package reads

import (
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/status"
)

// MapState is the phase 2 contract: every DBOS state lands on one of the
// five UI states, and unknown states are rejected, not guessed.
func TestMapState(t *testing.T) {
	for state, want := range map[dbos.WorkflowStatusType]status.State{
		dbos.WorkflowStatusEnqueued:                    status.Queued,
		dbos.WorkflowStatusDelayed:                     status.Queued,
		dbos.WorkflowStatusPending:                     status.Running,
		dbos.WorkflowStatusSuccess:                     status.Success,
		dbos.WorkflowStatusError:                       status.Error,
		dbos.WorkflowStatusCancelled:                   status.Cancelled,
		dbos.WorkflowStatusMaxRecoveryAttemptsExceeded: status.Error,
	} {
		got, err := MapState(state)
		if err != nil {
			t.Fatalf("MapState(%s): %v", state, err)
		}
		if got != want {
			t.Errorf("MapState(%s): got %s, want %s", state, got, want)
		}
	}

	if _, err := MapState(dbos.WorkflowStatusType("bogus")); err == nil {
		t.Error("MapState(bogus): expected error, got nil")
	}
}
