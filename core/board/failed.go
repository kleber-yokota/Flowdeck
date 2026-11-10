package board

import (
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// FailedRun is the row of the failed list (the failed page): one row per
// workflow whose latest run ended error or cancelled (the latest run wins —
// a later success removes the row), with the error message of the error
// rows (the batched error payload) and the empty message of the cancelled
// rows (no trace — the execution page has it). State is the mapped state
// (always error or cancelled); CreatedAt is the run's CreatedAt as
// RFC3339 (the web formats the "02/01 15:04" from it).
type FailedRun struct {
	Name      string
	ID        string
	State     string
	CreatedAt string
	Message   string
}

// Failed is the read of the failed workflows (the failed page): one row per
// workflow whose latest run is error or cancelled (F3 · the failed-box rule),
// in the SDK's desc order (newest first, no re-sort · F4). One list query
// (desc · LoadInput(false) · LoadOutput(false)) keeps the first row per name
// (the latest run · F1); the error rows then get the message from the
// batched error query (the error-row ids · LoadOutput(true) — the error
// column loads only with it · F2 · F9; the cancelled rows have no message).
// Not up: ErrNotLaunched (the guard, like every other read · F4).
func Failed() ([]FailedRun, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return nil, err
	}
	statuses, err := dbos.ListWorkflows(c,
		dbos.WithFilterSortDesc(),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(false),
	)
	if err != nil {
		return nil, fmt.Errorf("core: failed: %w", err)
	}
	return failedRows(c, statuses)
}

// failedRows keeps the first row per name (the latest run · F1) and
// attaches the error rows' messages from the batched error query (the
// error-row ids · LoadOutput(true) · F2 · F9; the cancelled rows have no
// message).
func failedRows(c dbos.Context, statuses []dbos.WorkflowStatus) ([]FailedRun, error) {
	runs, errorIDs, err := collectFailed(statuses)
	if err != nil {
		return nil, err
	}
	if len(errorIDs) == 0 {
		return runs, nil
	}
	msgs, err := errorMessages(c, errorIDs)
	if err != nil {
		return nil, err
	}
	for i := range runs {
		runs[i].Message = msgs[runs[i].ID]
	}
	return runs, nil
}

// collectFailed keeps the first row per name (the latest run · F1) and
// gathers the error-row ids for the batched error query.
func collectFailed(statuses []dbos.WorkflowStatus) ([]FailedRun, []string, error) {
	var runs []FailedRun
	seen := map[string]bool{}
	var errorIDs []string
	for _, s := range statuses {
		st, err := reads.MapState(s.Status)
		if err != nil {
			return nil, nil, err
		}
		if seen[s.Name] {
			continue // first row per name = the latest run (F1)
		}
		seen[s.Name] = true
		if !isFailed(st) {
			continue
		}
		run := FailedRun{
			Name:      s.Name,
			ID:        s.ID,
			State:     string(st),
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		}
		if st == status.Error {
			errorIDs = append(errorIDs, s.ID)
		}
		runs = append(runs, run)
	}
	return runs, errorIDs, nil
}

// errorMessages is the batched error payload: the list query's s.Error is
// nil without LoadOutput(true) (F2) — the message comes from the error-row
// ids only (the cancelled rows have no message · F9).
func errorMessages(c dbos.Context, ids []string) (map[string]string, error) {
	errs, err := dbos.ListWorkflows(c,
		dbos.WithFilterWorkflowIDs(ids...),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(true),
	)
	if err != nil {
		return nil, fmt.Errorf("core: failed: %w", err)
	}
	msgs := map[string]string{}
	for _, s := range errs {
		if s.Error != nil {
			msgs[s.ID] = s.Error.Error()
		}
	}
	return msgs, nil
}
