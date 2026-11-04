package reads

import (
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/status"
)

// LiveRun is the row of the runnings list: the live read of one running
// run (the rows are runs, not workflows — the same workflow running
// twice is two rows: no deduplication by workflow name). State is
// always the running state (the list filters to it); CreatedAt is the
// run's CreatedAt as RFC3339 (the web formats the "15:04" and the
// elapsed from it).
type LiveRun struct {
	Name      string
	ID        string
	State     string
	CreatedAt string
}

// Living is the live read of the running runs (the runnings page): one
// row per run running right now, in the SDK's desc order (newest first,
// no re-sort). One ListWorkflows (sorted desc — the same options as
// LiveCounts — and no input/output loaded: the rows only read the
// status, the name, the id and the created_at). Not up: ErrNotLaunched
// (the guard, like every other read).
func Living() ([]LiveRun, error) {
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
		return nil, fmt.Errorf("core: living: %w", err)
	}
	var runs []LiveRun
	for _, s := range statuses {
		st, err := MapState(s.Status)
		if err != nil {
			return nil, err
		}
		if st != status.Running {
			continue
		}
		runs = append(runs, LiveRun{
			Name:      s.Name,
			ID:        s.ID,
			State:     string(status.Running),
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}
	return runs, nil
}
