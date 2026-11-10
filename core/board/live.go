package board

import (
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// Live is the three live buckets of the dashboard (the live read): the
// runs running right now · the runs in the queue right now · the
// workflows whose latest run is failed (error or cancelled). A latest
// success with no live run is in no box.
type Live struct {
	Running int
	Queued  int
	Failed  int
}

// LiveCounts buckets the runs: the running and the queued boxes count
// every run in that state right now (firing the same workflow several
// times counts each run — no deduplication by workflow); the failed box
// counts the workflows whose latest run is error or cancelled. One
// ListWorkflows (sorted desc — the first row per name is the latest run —
// and no input/output loaded, the boxes only read the status column).
func LiveCounts() (Live, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return Live{}, err
	}
	statuses, err := dbos.ListWorkflows(c,
		dbos.WithFilterSortDesc(),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(false),
	)
	if err != nil {
		return Live{}, fmt.Errorf("core: live: %w", err)
	}
	return bucketLive(statuses)
}

// bucketLive buckets the rows (sorted desc — the first row per name is
// the latest run): the running and the queued boxes count every run in
// that state right now (firing the same workflow several times counts
// each run — no deduplication by workflow); the failed box counts the
// workflows whose latest run is error or cancelled.
func bucketLive(statuses []dbos.WorkflowStatus) (Live, error) {
	var live Live
	seen := map[string]bool{}
	for _, s := range statuses {
		st, err := reads.MapState(s.Status)
		if err != nil {
			return Live{}, err
		}
		switch st {
		case status.Running:
			live.Running++
		case status.Queued:
			live.Queued++
		}
		if !seen[s.Name] {
			seen[s.Name] = true // first row of the name: the latest run
			if isFailed(st) {
				live.Failed++
			}
		}
	}
	return live, nil
}

// isFailed is the failed box's rule: error or cancelled.
func isFailed(st status.State) bool {
	return st == status.Cancelled || st == status.Error
}
