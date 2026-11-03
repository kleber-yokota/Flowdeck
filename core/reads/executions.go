// Package reads is the execution reads of the core: the Executions (the
// listing of a workflow's runs), the Status (the badge state — the poll
// read), the Detail (the detail page's read model in one read) and the
// Steps (the executed path — the poll control). Nothing is executed here:
// the reads observe the durable state, the actions own the runs.
package reads

import (
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
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
	c, err := boot.CtxOrErr()
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
		st, err := MapState(s.Status)
		if err != nil {
			return nil, err
		}
		out = append(out, Execution{ID: s.ID, State: st, CreatedAt: s.CreatedAt})
	}
	return out, nil
}
