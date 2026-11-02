package reads

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/registry"
	"flowdeck/core/status"
)

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

// DetailPath is the read model of the execution detail page (phase 3,
// DETAIL · phase 4, STEP 3): the execution's row (the badge state, the
// first execution date, the attempts, the stored input values, and — on
// failure — the error and the failed step) and the executed path (the
// state + the steps that ran, in execution order) — one read: the status
// row, the step rows, and the child rows (only when the run dispatched a
// child). The state rides on the path (the poll control).
type DetailPath struct {
	ExecutionDetail
	RunPath
}

// Detail reads the execution detail page (phase 3, DETAIL: GET
// /executions/{id}) — one read: the status row (SELECT · workflow_id:
// status, retries, error, the stored input) and the step rows (SELECT
// STEPS · id: the step that errored, on failure, and the executed path).
func Detail(workflowID string) (DetailPath, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return DetailPath{}, err
	}
	statuses, err := dbos.ListWorkflows(c,
		dbos.WithFilterWorkflowIDs(workflowID),
		dbos.WithFilterLoadInput(true),
		dbos.WithFilterLoadOutput(true), // the error payload loads with the output
	)
	if err != nil {
		return DetailPath{}, fmt.Errorf("core: detail %s: %w", workflowID, err)
	}
	if len(statuses) == 0 {
		return DetailPath{}, ErrUnknownExecution
	}
	s := statuses[0]
	st, err := MapState(s.Status)
	if err != nil {
		return DetailPath{}, err
	}
	d := DetailPath{ExecutionDetail: ExecutionDetail{ID: s.ID, Name: s.Name, State: st, CreatedAt: s.CreatedAt, Attempts: s.Attempts}}
	d.Inputs = inputValues(s.Input, s.Name)
	if s.Error != nil {
		d.Error = s.Error.Error()
	}
	rows, err := dbos.GetWorkflowSteps(c, workflowID, dbos.WithStepsLoadOutput(false))
	if err != nil {
		return DetailPath{}, fmt.Errorf("core: detail path %s: %w", workflowID, err)
	}
	d.FailedStep = failedStepOf(rows)
	steps, err := stepRows(c, rows)
	if err != nil {
		return DetailPath{}, err
	}
	d.RunPath = RunPath{State: st, Steps: steps}
	return d, nil
}

// failedStepOf returns the name of the last step that errored ("" when
// none errored) — the detail's "failed step" row.
func failedStepOf(rows []dbos.StepInfo) string {
	failed := ""
	for _, row := range rows {
		if row.Error != nil {
			failed = row.StepName
		}
	}
	return failed
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
	spec, ok := registry.SpecOf(workflow)
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
