package reads

import (
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/status"
)

// The row kinds of the executed path (improve: a kind per row name) —
// the badge colors the row by name, not by state.
const (
	KindStep  = "step"
	KindChild = "child"
	KindWait  = "wait"
)

// Step is one row of the run's executed path (phase 4, STEP 3): the row
// name, its badge state (a row's error is a failed row, the rest are
// success rows) and its kind; for a child row — the child id and the
// child workflow's name and badge state (fetched from the child rows).
// StartedAt/CompletedAt are the row's times (a child row never completes
// — the wait row does).
type Step struct {
	Name        string
	State       status.State
	Kind        string
	ChildID     string
	ChildName   string
	ChildState  status.State
	StartedAt   time.Time
	CompletedAt time.Time
}

// childIDsOf returns the child workflow ids of the run's step rows (the
// rows whose child_workflow_id is set — the dispatch rows and the wait
// rows that follow them), deduplicated.
func childIDsOf(rows []dbos.StepInfo) []string {
	seen := map[string]struct{}{}
	for _, row := range rows {
		if row.ChildWorkflowID != "" {
			seen[row.ChildWorkflowID] = struct{}{}
		}
	}
	ids := make([]string, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	return ids
}

// childInfo is the child workflow's read model (the badge on the child
// row): the child's name and its badge state.
type childInfo struct {
	Name  string
	State status.State
}

// childRows reads the child workflow rows by their ids (the children
// dispatched by the run's RunWorkflow steps): the child id -> name + badge
// state map that colors the child rows.
func childRows(c dbos.Context, childIDs []string) (map[string]childInfo, error) {
	children, err := dbos.ListWorkflows(c, dbos.WithFilterWorkflowIDs(childIDs...))
	if err != nil {
		return nil, fmt.Errorf("core: child rows: %w", err)
	}
	out := make(map[string]childInfo, len(children))
	for _, s := range children {
		st, err := MapState(s.Status)
		if err != nil {
			return nil, err
		}
		out[s.ID] = childInfo{Name: s.Name, State: st}
	}
	return out, nil
}

// kindOf maps a row onto its kind (improve): the wait-row names (the
// SDK-internal rows — DBOS.getResult · DBOS.select · DBOS.recv ·
// DBOS.sleep, fact.md row 4) are waits; a row whose child_workflow_id is
// set is the dispatch of a child; the rest are steps. The wait names are
// checked first: the DBOS.getResult row carries the child id of its child
// row, but it is the wait, not the child.
func kindOf(stepName string, childID string) string {
	switch stepName {
	case "DBOS.getResult", "DBOS.select", "DBOS.recv", "DBOS.sleep":
		return KindWait
	}
	if childID != "" {
		return KindChild
	}
	return KindStep
}

// stepRow maps a step row onto a Step line: the state is the row's error
// (an error is a failed row, the rest are success rows), the kind by the
// row's name (kindOf), and — for a child row — the child workflow's name
// and badge state from the child rows map.
func stepRow(row dbos.StepInfo, children map[string]childInfo) Step {
	st := status.Success
	if row.Error != nil {
		st = status.Error
	}
	s := Step{
		Name:        row.StepName,
		State:       st,
		Kind:        kindOf(row.StepName, row.ChildWorkflowID),
		ChildID:     row.ChildWorkflowID,
		StartedAt:   row.StartedAt,
		CompletedAt: row.CompletedAt,
	}
	if s.Kind == KindChild {
		if child, ok := children[s.ChildID]; ok {
			s.ChildName = child.Name
			s.ChildState = child.State
		}
	}
	return s
}

// RunPath is the read model of the executed path of a run (phase 4,
// STEP 3): the run's state (the poll control — a terminal state stops the
// 2s polling of the fragment) and the steps that ran, in execution order.
type RunPath struct {
	State status.State
	Steps []Step
}

// Steps returns the executed path of a run: the state and the steps that
// ran, in execution order (step_id asc — the branch of the if the run
// took), read from the step rows in Postgres (phase 4, STEP 3:
// GET /executions/{id}/steps → STEPS(id) → SELECT status · steps · id). It
// is separate from the boot map of possible paths: the map is code, the
// path is what ran. The row mapping (stepRows, the batched child query) is
// the one the detail page's read (DetailPath) shares.
func Steps(workflowID string) (RunPath, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return RunPath{}, err
	}
	statuses, err := ListByID(c, workflowID)
	if err != nil {
		return RunPath{}, fmt.Errorf("core: steps %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return RunPath{}, ErrUnknownExecution
	}
	state, err := MapState(statuses[0].Status)
	if err != nil {
		return RunPath{}, err
	}
	rows, err := dbos.GetWorkflowSteps(c, workflowID, dbos.WithStepsLoadOutput(false))
	if err != nil {
		return RunPath{}, fmt.Errorf("core: steps %s: %w", workflowID, err)
	}
	steps, err := stepRows(c, rows)
	if err != nil {
		return RunPath{}, err
	}
	return RunPath{State: state, Steps: steps}, nil
}

// stepRows maps the run's step rows onto Step lines (Postgres returns the
// rows step_id asc — the branch the run took): the child rows are one
// batched query by the child ids, only when the run dispatched a child.
func stepRows(c dbos.Context, rows []dbos.StepInfo) ([]Step, error) {
	children := map[string]childInfo{}
	if ids := childIDsOf(rows); len(ids) > 0 {
		ch, err := childRows(c, ids)
		if err != nil {
			return nil, err
		}
		children = ch
	}
	steps := make([]Step, 0, len(rows))
	for _, row := range rows {
		steps = append(steps, stepRow(row, children))
	}
	return steps, nil
}

// StepDetail is the read model of one step row of a run (STEP 1 — the
// expand fragment's read): the stored output of the step (the JSON text
// of its result — a string result arrives quoted) and, for a failed
// step, its error (no output: the step did not return).
type StepDetail struct {
	Output string
	Error  string
}

// StepDetailOf reads the stored output or the error of one step of a run
// by name (STEP 1, GET /executions/{id}/steps/{step}): one per-run
// GetWorkflowSteps with LoadOutput(true) (the output text needs it —
// fact 6), the row resolved by name (a repeated name — a loop iteration
// — is the last row). An unknown run is ErrUnknownExecution; a step the
// run never executed is a wrapped error (no new sentinel — the web's
// Fail maps it to its 500 default).
func StepDetailOf(workflowID, step string) (StepDetail, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return StepDetail{}, err
	}
	statuses, err := ListByID(c, workflowID)
	if err != nil {
		return StepDetail{}, fmt.Errorf("core: step %s in %s: %w", step, workflowID, err)
	}
	if len(statuses) == 0 {
		return StepDetail{}, ErrUnknownExecution
	}
	rows, err := dbos.GetWorkflowSteps(c, workflowID, dbos.WithStepsLoadOutput(true))
	if err != nil {
		return StepDetail{}, fmt.Errorf("core: step %s in %s: %w", step, workflowID, err)
	}
	found, ok := stepRowBy(rows, step)
	if !ok {
		return StepDetail{}, fmt.Errorf("core: step %q not found in run %s", step, workflowID)
	}
	return stepDetailFrom(found), nil
}

// stepRowBy resolves the run's step rows by name (a repeated name — a loop
// iteration — is the last row).
func stepRowBy(rows []dbos.StepInfo, name string) (*dbos.StepInfo, bool) {
	var found *dbos.StepInfo
	for i := range rows {
		if rows[i].StepName == name {
			found = &rows[i]
		}
	}
	return found, found != nil
}

// stepDetailFrom maps one step row to the read model: a failed step
// exposes its error, not its output (the SDK stores the JSON text "" for a
// step that did not return — fact 14, an artifact, not displayable
// output); otherwise the stored output (the JSON text — a string result
// arrives quoted).
func stepDetailFrom(row *dbos.StepInfo) StepDetail {
	d := StepDetail{}
	if row.Error != nil {
		d.Error = row.Error.Error()
		return d
	}
	if s, ok := jsonText(row.Output); ok {
		d.Output = s
	}
	return d
}
