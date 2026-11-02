package core

import (
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/status"
)

// Execution is one run of a workflow as listed on the per-workflow page
// (phase 3, LIST): the workflow ID, the badge state and the first
// execution date (created_at). The list is newest first.
type Execution struct {
	ID        string
	State     status.State
	CreatedAt time.Time
}

// Executions lists the runs of the named workflow, newest first (phase 3,
// LIST: GET /workflows/{name} → LIST(name) → the web renders one row per
// run: workflow ID + badge).
func Executions(name string) ([]Execution, error) {
	c, err := ctxOrErr()
	if err != nil {
		return nil, err
	}
	statuses, err := dbos.ListWorkflows(c,
		dbos.WithFilterName(name),
		dbos.WithFilterSortDesc(),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(false),
	)
	if err != nil {
		return nil, fmt.Errorf("core: executions %s: %w", name, err)
	}
	out := make([]Execution, 0, len(statuses))
	for _, s := range statuses {
		st, err := mapState(s.Status)
		if err != nil {
			return nil, err
		}
		out = append(out, Execution{ID: s.ID, State: st, CreatedAt: s.CreatedAt})
	}
	return out, nil
}
