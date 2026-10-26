package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
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

// ExecutionDetail is the read model of a single execution (phase 3,
// DETAIL): the badge state, the first execution date (created_at), the
// attempts, the stored input values (phase 5 — the input is written once
// at enqueue, immutable per workflow_id), and — on failure — the error
// and the step that failed.
type ExecutionDetail struct {
	ID         string
	Name       string
	State      status.State
	CreatedAt  time.Time
	Attempts   int
	Inputs     []InputValue
	Error      string
	FailedStep string
}

// InputValue is one stored input value of an execution (the detail shows
// the values the run used).
type InputValue struct {
	Name  string
	Value string
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
	State status.State
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
		st := status.Success
		if row.Error != nil {
			st = status.Error
		}
		steps = append(steps, Step{Name: row.StepName, State: st})
	}
	return steps, nil
}

// Detail reads one execution (phase 3, DETAIL: GET /executions/{id}) — the
// status row (SELECT · workflow_id: status, retries, error, the stored
// input) and the step rows (SELECT STEPS · id: the step that errored, on
// failure).
func Detail(workflowID string) (ExecutionDetail, error) {
	c, err := ctxOrErr()
	if err != nil {
		return ExecutionDetail{}, err
	}
	statuses, err := dbos.ListWorkflows(c,
		dbos.WithFilterWorkflowIDs(workflowID),
		dbos.WithFilterLoadInput(true),
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
	d.Inputs = inputValues(s.Input, s.Name)
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

// inputValues maps the stored input (DBOS keeps it as the JSON text of the
// input) onto the display pairs (phase 5, DETAIL): a struct input is the
// map of field → value (sorted by field name); a simple input is the
// bare value under the input's name (a JSON string is unquoted).
func inputValues(raw any, workflow string) []InputValue {
	if raw == nil {
		return nil // no input (a dispatch without a form)
	}
	in, ok := inputOf(workflow)
	if !ok {
		return nil
	}
	if m := structValues(raw); m != nil {
		return m
	}
	if s, isText := jsonText(raw); isText {
		if quoted, ok := quotedString(s); ok {
			return []InputValue{{Name: in, Value: quoted}}
		}
		return []InputValue{{Name: in, Value: s}}
	}
	return []InputValue{{Name: in, Value: fmt.Sprint(raw)}}
}

// inputOf is the input name of the workflow (the first input — the simple
// case; a struct input is displayed field by field, the name is unused).
func inputOf(workflow string) (string, bool) {
	spec, ok := specOf(workflow)
	if !ok || len(spec.Inputs) == 0 {
		return "", false
	}
	return spec.Inputs[0].Name, true
}

// structValues is the sorted map of field → value of the stored struct
// input (the JSON text of the map) — nil for anything that is not the
// JSON of a map.
func structValues(raw any) []InputValue {
	s, isText := jsonText(raw)
	if !isText || !strings.HasPrefix(s, "{") {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil
	}
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	out := make([]InputValue, 0, len(m))
	for _, name := range names {
		out = append(out, InputValue{Name: name, Value: fmt.Sprint(m[name])})
	}
	return out
}

// quotedString unquotes a stored JSON string (a simple input) — ok is
// false for anything that is not the JSON of a string.
func quotedString(s string) (string, bool) {
	if !strings.HasPrefix(s, "\"") {
		return "", false
	}
	var quoted string
	if err := json.Unmarshal([]byte(s), &quoted); err != nil {
		return "", false
	}
	return quoted, true
}

func jsonText(raw any) (string, bool) {
	switch v := raw.(type) {
	case string:
		return v, true
	case []byte:
		return string(v), true
	case json.RawMessage:
		return string(v), true
	}
	return "", false
}
