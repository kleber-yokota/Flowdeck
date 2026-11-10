package board

import (
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// Queued is the live read of the queued runs (the queued page): one row
// per queued run (the ENQUEUED ∪ DELAYED bucket via MapState), in the
// SDK's desc order (newest first, no re-sort). The mirror of Living: the
// same ListWorkflows (sorted desc — the same options as Living — and no
// input/output loaded: the rows only read the status, the name, the id
// and the created_at). The rows are runs, not workflows: the same
// workflow queued twice is two rows (no deduplication by workflow name).
// The row State is always the queued state (the list filters to it);
// CreatedAt is the run's CreatedAt as RFC3339 (the web formats the
// "15:04" and the elapsed from it). Not up: ErrNotLaunched (the guard,
// like every other read).
func Queued() ([]LiveRun, error) {
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
		return nil, fmt.Errorf("core: queued: %w", err)
	}
	var runs []LiveRun
	for _, s := range statuses {
		st, err := reads.MapState(s.Status)
		if err != nil {
			return nil, err
		}
		if st != status.Queued {
			continue
		}
		runs = append(runs, LiveRun{
			Name:      s.Name,
			ID:        s.ID,
			State:     string(status.Queued),
			CreatedAt: s.CreatedAt.Format(time.RFC3339),
		})
	}
	return runs, nil
}
