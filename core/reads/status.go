package reads

import (
	"errors"
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/status"
)

// ErrUnknownExecution is returned by Status when the workflow ID is not
// known by the core (the web maps it to a 404).
var ErrUnknownExecution = errors.New("core: unknown workflow execution")

// ListByID lists the status rows of one execution without loading
// input/output: only the status row is needed (the 2s badge poll must
// stay cheap).
func ListByID(c dbos.Context, workflowID string) ([]dbos.WorkflowStatus, error) {
	return dbos.ListWorkflows(c,
		dbos.WithFilterWorkflowIDs(workflowID),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(false),
	)
}

// Status maps the DBOS state of a run onto the UI states:
//
//	ENQUEUED/DELAYED -> queued, PENDING -> running,
//	SUCCESS -> success, ERROR/MAX_RECOVERY_ATTEMPTS_EXCEEDED -> error,
//	CANCELLED -> cancelled
//
// It is the read half of the dispatch (phase 2): the web polls it every 2s
// and swaps the badge. Nothing is executed here — the DBOS executor owns
// the run, this only observes its durable state in the system database.
func Status(workflowID string) (status.State, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return "", err
	}
	statuses, err := ListByID(c, workflowID)
	if err != nil {
		return "", fmt.Errorf("core: status %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return "", ErrUnknownExecution
	}
	return MapState(statuses[0].Status)
}

// MapState maps a DBOS workflow state onto the UI states (the phase 2
// contract: every DBOS state lands on one of the five, unknown states are
// rejected, not guessed).
func MapState(s dbos.WorkflowStatusType) (status.State, error) {
	switch s {
	case dbos.WorkflowStatusEnqueued, dbos.WorkflowStatusDelayed:
		return status.Queued, nil
	case dbos.WorkflowStatusPending:
		return status.Running, nil
	case dbos.WorkflowStatusSuccess:
		return status.Success, nil
	case dbos.WorkflowStatusCancelled:
		return status.Cancelled, nil
	case dbos.WorkflowStatusError, dbos.WorkflowStatusMaxRecoveryAttemptsExceeded:
		return status.Error, nil
	default:
		return "", fmt.Errorf("core: unmapped workflow state: %s", s)
	}
}
