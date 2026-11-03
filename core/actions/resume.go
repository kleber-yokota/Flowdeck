package actions

import (
	"errors"
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// ErrNotResumable is returned by Resume when the run is not failed or
// cancelled (only failed and cancelled runs can be resumed). The web maps
// it to a 400.
var ErrNotResumable = errors.New("core: run cannot be resumed")

// Resume re-enqueues a failed or cancelled run by id (phase 5, RESUME):
// the cancelled run is re-enqueued by the SDK resume (it resumes after
// the last completed step); the failed run is rewound — the SDK deletes
// the step rows from the first failed row onward (the cut), keeps the
// stored input, resets recovery_attempts, and re-enqueues the same id on
// the same queue (the SDK resume is a silent no-op on ERROR rows — the
// resume SQL excludes SUCCESS and ERROR). The ACK is the row read back:
// the same id, now enqueued again.
func Resume(workflowID string) (reads.Execution, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return reads.Execution{}, err
	}
	st, err := reads.Status(workflowID)
	if err != nil {
		return reads.Execution{}, err
	}
	if st != status.Error && st != status.Cancelled {
		return reads.Execution{}, fmt.Errorf("%w: state %s", ErrNotResumable, st)
	}
	// The cancelled row: the SDK resume re-enqueues it (its SQL re-enqueues
	// CANCELLED rows) — it resumes after the last completed step.
	if st == status.Cancelled {
		if _, err := dbos.ResumeWorkflow[any](c, workflowID, dbos.WithResumeQueue(boot.QueueName)); err != nil {
			return reads.Execution{}, fmt.Errorf("core: resume %s: %w", workflowID, err)
		}
		return resumeAck(c, workflowID)
	}
	// The failed row: the SDK resume no-ops on ERROR rows — the run is
	// rewound instead (the cut, the first failed step row).
	cut, err := rewindCut(workflowID)
	if err != nil {
		return reads.Execution{}, err
	}
	if _, err := dbos.RewindWorkflow[any](c, workflowID, dbos.WithRewindQueue(boot.QueueName), dbos.WithRewindStartStep(cut)); err != nil {
		return reads.Execution{}, fmt.Errorf("core: resume %s: %w", workflowID, err)
	}
	return resumeAck(c, workflowID)
}

// rewindCut is the 0-based index of the first failed step row (the rows of
// reads.Steps are in function_id order): the SDK deletes the step rows from
// the cut onward. A failed run always has its failed row — the end of the
// path is the defensive default when none is found.
func rewindCut(workflowID string) (uint, error) {
	path, err := reads.Steps(workflowID)
	if err != nil {
		return 0, fmt.Errorf("core: resume %s: %w", workflowID, err)
	}
	cut := uint(len(path.Steps))
	for i, s := range path.Steps {
		if s.State == status.Error {
			cut = uint(i)
			break
		}
	}
	return cut, nil
}

// resumeAck reads the resumed row back: the same id, now enqueued again
// (the resume never touches the stored input). A row that stayed in a
// failed state (a terminal error — the resume did not take) is not
// resumable.
func resumeAck(c dbos.Context, workflowID string) (reads.Execution, error) {
	statuses, err := reads.ListByID(c, workflowID)
	if err != nil {
		return reads.Execution{}, fmt.Errorf("core: resume %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return reads.Execution{}, reads.ErrUnknownExecution
	}
	st, err := reads.MapState(statuses[0].Status)
	if err != nil {
		return reads.Execution{}, err
	}
	if st == status.Error {
		return reads.Execution{}, fmt.Errorf("%w: state %s", ErrNotResumable, st)
	}
	return reads.Execution{ID: workflowID, State: st, CreatedAt: statuses[0].CreatedAt}, nil
}
