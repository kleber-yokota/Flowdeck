package actions

import (
	"errors"
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// ErrNotCancellable is returned by Cancel when the run is not live (only
// queued/running runs can be cancelled — a terminal run is already
// settled). The web maps it to a 400.
var ErrNotCancellable = errors.New("core: run cannot be cancelled")

// Cancel cancels a live run by id (the cancel sequence): the guard lets
// only queued/running through, the DBOS cancel flips the row to CANCELLED
// and takes it out of the queue (a running step completes but the run
// stops at the next step), and the ACK reads the row back — the same id,
// now cancelled.
func Cancel(workflowID string) (reads.Execution, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return reads.Execution{}, err
	}
	st, err := reads.Status(workflowID)
	if err != nil {
		return reads.Execution{}, err
	}
	if st != status.Queued && st != status.Running {
		return reads.Execution{}, fmt.Errorf("%w: state %s", ErrNotCancellable, st)
	}
	if err := dbos.CancelWorkflow(c, workflowID); err != nil {
		return reads.Execution{}, fmt.Errorf("core: cancel %s: %w", workflowID, err)
	}
	return cancelAck(c, workflowID)
}

// cancelAck reads the cancelled row back: the same id, now cancelled
// (the SDK returns only once the row is CANCELLED).
func cancelAck(c dbos.Context, workflowID string) (reads.Execution, error) {
	statuses, err := reads.ListByID(c, workflowID)
	if err != nil {
		return reads.Execution{}, fmt.Errorf("core: cancel %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return reads.Execution{}, reads.ErrUnknownExecution
	}
	st, err := reads.MapState(statuses[0].Status)
	if err != nil {
		return reads.Execution{}, err
	}
	return reads.Execution{ID: workflowID, State: st, CreatedAt: statuses[0].CreatedAt}, nil
}
