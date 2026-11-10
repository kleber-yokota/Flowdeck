// Package reads is the execution reads of the core: the Executions (the
// listing of a workflow's runs), the Status (the badge state — the poll
// read), the Detail (the detail page's read model in one read) and the
// Steps (the executed path — the poll control). Nothing is executed here:
// the reads observe the durable state, the actions own the runs.
package reads

import (
	"encoding/json"
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/status"
)

// Execution is one run of a workflow as listed on the per-workflow page
// (phase 3, LIST): the workflow ID, the badge state, the first execution
// date (created_at), the status update time (updated_at — the terminal
// completion time) and the terminal row fields (the decoded output and
// the error message). The list is newest first.
type Execution struct {
	ID        string
	State     status.State
	CreatedAt time.Time
	UpdatedAt time.Time
	Output    string
	Error     string
}

// ExecutionPage is one page of the run listing (the console pages the
// runs: limit per page, newest first, and the HasMore flag that drives
// the pager — no total).
type ExecutionPage struct {
	Rows    []Execution
	HasMore bool
}

// Executions lists one page of the runs of the named workflow, newest
// first (phase 3, LIST → the console: GET /workflows/{name}?page=N):
// limit rows from the offset (before) — the SDK pages by limit + offset
// (fact F1), so the read fetches limit+1 and the surplus is the HasMore
// flag, without a total.
func Executions(name string, limit int, before int) (ExecutionPage, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return ExecutionPage{}, err
	}
	statuses, err := dbos.ListWorkflows(c,
		dbos.WithFilterName(name),
		dbos.WithFilterSortDesc(),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(true),
		dbos.WithFilterLimit(limit+1),
		dbos.WithFilterOffset(before),
	)
	if err != nil {
		return ExecutionPage{}, fmt.Errorf("core: executions %s: %w", name, err)
	}
	rows := make([]Execution, 0, len(statuses))
	for _, s := range statuses {
		st, err := MapState(s.Status)
		if err != nil {
			return ExecutionPage{}, err
		}
		e := Execution{
			ID:        s.ID,
			State:     st,
			CreatedAt: s.CreatedAt,
			UpdatedAt: s.UpdatedAt,
			Output:    outputText(s.Output),
		}
		if s.Error != nil {
			e.Error = s.Error.Error()
		}
		rows = append(rows, e)
	}
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}
	return ExecutionPage{Rows: rows, HasMore: hasMore}, nil
}

// outputText decodes the run's stored output for display: the SDK's
// public read returns the raw JSON text of the result as a plain string
// (fact F3 — a string result ok arrives as "ok", with the quotes), so the
// read json-unmarshals it. A missing or non-string value renders empty.
func outputText(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	var out string
	if err := json.Unmarshal([]byte(s), &out); err != nil {
		return ""
	}
	return out
}
