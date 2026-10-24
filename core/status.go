package core

import (
	"errors"
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// State is the UI state of a workflow run — one of the four badge colors
// of the dashboard.
type State string

// The four execution states, in the order the run moves through them:
//
//	queued -> running -> success | error
const (
	StateQueued  State = "queued"  // in queue  (#7cc0ff)
	StateRunning State = "running" // running   (#ff9142)
	StateSuccess State = "success" // success   (#46d19a)
	StateError   State = "error"   // failed    (#ff5450)
)

// ErrUnknownExecution is returned by Status when the workflow ID is not
// known by the core (the web maps it to a 404).
var ErrUnknownExecution = errors.New("core: unknown workflow execution")

// Status maps the DBOS state of a run onto the UI states:
//
//	ENQUEUED/DELAYED -> queued, PENDING -> running,
//	SUCCESS -> success, ERROR/CANCELLED -> error.
//
// It is the read half of the dispatch (phase 2): the web polls it every 2s
// and swaps the badge. Nothing is executed here — the DBOS executor owns
// the run, this only observes its durable state in the system database.
func Status(workflowID string) (State, error) {
	mu.RLock()
	c := dbosCtx
	up := booted && launched
	mu.RUnlock()

	if !up || c == nil {
		return "", ErrNotLaunched
	}

	// The badge poll hits this every 2s per open execution, so skip
	// input/output loading: only the status row is needed.
	statuses, err := dbos.ListWorkflows(c,
		dbos.WithFilterWorkflowIDs(workflowID),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(false),
	)
	if err != nil {
		return "", fmt.Errorf("core: status %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return "", ErrUnknownExecution
	}
	return mapState(statuses[0].Status)
}

// mapState maps a DBOS workflow state onto the UI states.
func mapState(s dbos.WorkflowStatusType) (State, error) {
	switch s {
	case dbos.WorkflowStatusEnqueued, dbos.WorkflowStatusDelayed:
		return StateQueued, nil
	case dbos.WorkflowStatusPending:
		return StateRunning, nil
	case dbos.WorkflowStatusSuccess:
		return StateSuccess, nil
	case dbos.WorkflowStatusError, dbos.WorkflowStatusCancelled:
		return StateError, nil
	default:
		return "", fmt.Errorf("core: unmapped workflow state: %s", s)
	}
}
