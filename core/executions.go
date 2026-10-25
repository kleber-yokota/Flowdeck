package core

import (
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// Execution is one run of a workflow as listed on the per-workflow page
// (phase 3, LIST): the workflow ID, the badge state and the first
// execution date (created_at). The list is newest first.
type Execution struct {
	ID        string
	State     State
	CreatedAt time.Time
}

// ExecutionDetail is the read model of a single execution (phase 3,
// DETAIL): the badge state, the first execution date (created_at), the
// attempts, and — on failure — the error and the step that failed.
type ExecutionDetail struct {
	ID         string
	Name       string
	State      State
	CreatedAt  time.Time
	Attempts   int
	Error      string
	FailedStep string
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

// Step is one step of the executed path of a run (phase 4, STEP 3): the
// step name and its badge state (success/failed — every executed step is
// terminal, the rows exist only once the step completed).
type Step struct {
	Name  string
	State State
}

// Steps returns the executed path of a run: the steps that ran, in
// execution order (step_id asc — the branch of the if the run took), read
// from the step rows in Postgres (phase 4, STEP 3: GET /executions/{id} →
// STEPS(id) → SELECT steps · id). It is separate from the boot map of
// possible paths: the map is code, the path is what ran.
func Steps(workflowID string) ([]Step, error) {
	c, err := ctxOrErr()
	if err != nil {
		return nil, err
	}
	statuses, err := listByID(c, workflowID)
	if err != nil {
		return nil, fmt.Errorf("core: steps %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return nil, ErrUnknownExecution
	}
	rows, err := dbos.GetWorkflowSteps(c, workflowID, dbos.WithStepsLoadOutput(false))
	if err != nil {
		return nil, fmt.Errorf("core: steps %s: %w", workflowID, err)
	}
	steps := make([]Step, 0, len(rows))
	for _, row := range rows { // Postgres returns the rows step_id asc
		st := StateSuccess
		if row.Error != nil {
			st = StateError
		}
		steps = append(steps, Step{Name: row.StepName, State: st})
	}
	return steps, nil
}

// Detail reads one execution (phase 3, DETAIL: GET /executions/{id}) — the
// status row (SELECT · workflow_id: status, retries, error) and the step
// rows (SELECT STEPS · id: the step that errored, on failure).
func Detail(workflowID string) (ExecutionDetail, error) {
	c, err := ctxOrErr()
	if err != nil {
		return ExecutionDetail{}, err
	}
	statuses, err := dbos.ListWorkflows(c,
		dbos.WithFilterWorkflowIDs(workflowID),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(true), // the error payload loads with the output
	)
	if err != nil {
		return ExecutionDetail{}, fmt.Errorf("core: detail %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return ExecutionDetail{}, ErrUnknownExecution
	}
	s := statuses[0]
	st, err := mapState(s.Status)
	if err != nil {
		return ExecutionDetail{}, err
	}
	d := ExecutionDetail{ID: s.ID, Name: s.Name, State: st, CreatedAt: s.CreatedAt, Attempts: s.Attempts}
	if s.Error != nil {
		d.Error = s.Error.Error()
	}
	steps, err := dbos.GetWorkflowSteps(c, workflowID, dbos.WithStepsLoadOutput(false))
	if err != nil {
		return ExecutionDetail{}, fmt.Errorf("core: detail steps %s: %w", workflowID, err)
	}
	for _, step := range steps { // the last step that errored is the failed step
		if step.Error != nil {
			d.FailedStep = step.StepName
		}
	}
	return d, nil
}
