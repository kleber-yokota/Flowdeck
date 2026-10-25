package core

import (
	"testing"

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
