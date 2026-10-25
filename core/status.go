package core

import (
	"errors"
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/status"
)

// ErrUnknownExecution is returned by Status when the workflow ID is not
// known by the core (the web maps it to a 404).
var ErrUnknownExecution = errors.New("core: unknown workflow execution")

// Status maps the DBOS state of a run onto the UI states:
//
//	ENQUEUED/DELAYED -> queued, PENDING -> running,
//	SUCCESS -> success, ERROR/CANCELLED/MAX_RECOVERY_ATTEMPTS_EXCEEDED -> error
//
// It is the read half of the dispatch (phase 2): the web polls it every 2s
// and swaps the badge. Nothing is executed here — the DBOS executor owns
// the run, this only observes its durable state in the system database.
// ctxOrErr returns the live DBOS context, or ErrNotLaunched — the read
// guard of every query (the core must be booted and launched).
func ctxOrErr() (dbos.Context, error) {
	theCore.mu.RLock()
	defer theCore.mu.RUnlock()
	if !theCore.booted || !theCore.launched || theCore.dbosCtx == nil {
		return nil, ErrNotLaunched
	}
	return theCore.dbosCtx, nil
}

// listByID lists the status rows of one execution without loading
// input/output: only the status row is needed (the 2s badge poll must
// stay cheap).
func listByID(c dbos.Context, workflowID string) ([]dbos.WorkflowStatus, error) {
	return dbos.ListWorkflows(c,
		dbos.WithFilterWorkflowIDs(workflowID),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(false),
	)
}

func Status(workflowID string) (status.State, error) {
	c, err := ctxOrErr()
	if err != nil {
		return "", err
	}
	statuses, err := listByID(c, workflowID)
	if err != nil {
		return "", fmt.Errorf("core: status %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return "", ErrUnknownExecution
	}
	return mapState(statuses[0].Status)
}

// mapState maps a DBOS workflow state onto the UI states (the phase 2
// contract: every DBOS state lands on one of the four, unknown states are
// rejected, not guessed).
func mapState(s dbos.WorkflowStatusType) (status.State, error) {
	switch s {
	case dbos.WorkflowStatusEnqueued, dbos.WorkflowStatusDelayed:
		return status.Queued, nil
	case dbos.WorkflowStatusPending:
		return status.Running, nil
	case dbos.WorkflowStatusSuccess:
		return status.Success, nil
	case dbos.WorkflowStatusError, dbos.WorkflowStatusCancelled, dbos.WorkflowStatusMaxRecoveryAttemptsExceeded:
		return status.Error, nil
	default:
		return "", fmt.Errorf("core: unmapped workflow state: %s", s)
	}
}
