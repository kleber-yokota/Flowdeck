package core

import (
	"errors"
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/status"
)

// ErrNotResumable is returned by Resume when the run is not failed (only
// failed runs can be resumed — the DBOS resume re-enqueues them). The web
// maps it to a 400.
var ErrNotResumable = errors.New("core: run cannot be resumed")

// Resume re-enqueues a failed run by id (phase 5, RESUME): the DBOS resume
// puts the run back on the queue with the same stored input — the run
// restarts from the last completed step. The ACK is the row read back:
// the same id, now enqueued again.
func Resume(workflowID string) (Execution, error) {
	c, err := ctxOrErr()
	if err != nil {
		return Execution{}, err
	}
	st, err := Status(workflowID)
	if err != nil {
		return Execution{}, err
	}
	if st != status.Error {
		return Execution{}, fmt.Errorf("%w: state %s", ErrNotResumable, st)
	}
	if _, err := dbos.ResumeWorkflow[any](c, workflowID, dbos.WithResumeQueue(QueueName)); err != nil {
		return Execution{}, fmt.Errorf("core: resume %s: %w", workflowID, err)
	}
	return resumeAck(c, workflowID)
}

// resumeAck reads the resumed row back: the same id, now enqueued again
// (the resume never touches the stored input). A row that stayed in a
// failed state (a terminal error — the resume did not take) is not
// resumable.
func resumeAck(c dbos.Context, workflowID string) (Execution, error) {
	statuses, err := listByID(c, workflowID)
	if err != nil {
		return Execution{}, fmt.Errorf("core: resume %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return Execution{}, ErrUnknownExecution
	}
	st, err := mapState(statuses[0].Status)
	if err != nil {
		return Execution{}, err
	}
	if st == status.Error {
		return Execution{}, fmt.Errorf("%w: state %s", ErrNotResumable, st)
	}
	return Execution{ID: workflowID, State: st, CreatedAt: statuses[0].CreatedAt}, nil
}
