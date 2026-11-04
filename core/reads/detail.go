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

// ExecutionDetail is the read model of a single execution: the badge
// state, the first execution date (created_at), the attempts (total: the
// dispatches so far — a run that is enqueued is 1, the first attempt; the
// display renders attempts − 1 recovered failures), the stored input
// values (the input is written once at enqueue, immutable per
// workflow_id), the children (one entry per child workflow of the run,
// sorted by id), and — on failure — the error and the step that failed.
type ExecutionDetail struct {
	ID         string
	Name       string
	State      status.State
	CreatedAt  time.Time
	Attempts   int
	Inputs     []InputValue
	Children   []ChildRef
	Error      string
	FailedStep string
}

// ChildRef is one child workflow of the run: the child id, the child's
// name, and the child's badge state (the child rows read by the child ids
// — the same read the step rows use).
type ChildRef struct {
	ID    string
	Name  string
	State status.State
}

// InputValue is one stored input value of an execution (the detail shows
// the values the run used).
type InputValue struct {
	Name  string
	Value string
}

// DetailPath is the read model of the execution detail page: the
// execution's row (the badge state, the first execution date, the
// attempts, the stored input values, and — on failure — the error and the
// failed step) and the executed path (the state + the steps that ran, in
// execution order) — one read: the status row, the step rows, and the
// child rows (only when the run dispatched a child). The state rides on
// the path (the poll control).
type DetailPath struct {
	ExecutionDetail
	RunPath
}

// Detail reads the execution detail page (GET /executions/{id}) — one
// read: the status row (SELECT · workflow_id:
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
	d := DetailPath{ExecutionDetail: ExecutionDetail{ID: s.ID, Name: s.Name, State: st, CreatedAt: s.CreatedAt, Attempts: displayAttempts(s.Attempts)}}
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
	d.Children, err = childrenOf(c, rows)
	if err != nil {
		return DetailPath{}, err
	}
	return d, nil
}

// displayAttempts is the display's attempts (the dispatches so far) from
// the SDK's attempts: the status row counts the current claim (0 until
// the first claim, 1 once claimed — a resume re-enqueues and starts the
// count again), and an enqueued run has its first attempt scheduled, so
// the display minimum is 1.
func displayAttempts(attempts int) int {
	if attempts < 1 {
		return 1
	}
	return attempts
}

// childrenOf lists the children of the run: one entry per child workflow,
// sorted by id (deterministic) — the child rows are the batched query by
// the child ids (the same read the step rows use).
func childrenOf(c dbos.Context, rows []dbos.StepInfo) ([]ChildRef, error) {
	ids := childIDsOf(rows)
	if len(ids) == 0 {
		return nil, nil
	}
	ch, err := childRows(c, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ChildRef, 0, len(ch))
	for id, child := range ch {
		out = append(out, ChildRef{ID: id, Name: child.Name, State: child.State})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
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

// inputValues maps the stored input (DBOS keeps it as the JSON text of
// the input) onto the display pairs: a struct input is the map of field
// → value (sorted by field name); a simple input is the bare value under
// the input's name (a JSON string is unquoted).
func inputValues(raw any, workflow string) []InputValue {
	if raw == nil {
		return nil // no input (a dispatch without a form)
	}
	in, ok := inputOf(workflow)
	if !ok {
		return nil
	}
	if m, isStruct := structValues(raw, workflow); isStruct {
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

// structValues is the field → value of the stored struct input (the JSON
// text of the map): every field of the spec, the stored value when the
// dispatch posted it, the Go zero value when it did not (the run received
// the zero). ok is false for anything that is not the JSON of a map.
func structValues(raw any, workflow string) ([]InputValue, bool) {
	m, ok := storedMap(raw)
	if !ok {
		return nil, false
	}
	return structInputValues(m, workflow), true
}

// storedMap is the stored input as a map (the JSON text that starts with
// "{").
func storedMap(raw any) (map[string]any, bool) {
	s, isText := jsonText(raw)
	if !isText || !strings.HasPrefix(s, "{") {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		return nil, false
	}
	return m, true
}

// inputField is one field of a struct input: its name and its type (the
// parser supports string and int).
type inputField struct{ name, typ string }

// structInputValues is the sorted (by name) field → value of a stored
// struct input: every field of the spec (the parsed source is the source
// of truth of the struct's fields), the stored value when the dispatch
// posted it, the Go zero value when it did not (the run received the
// zero). The stored keys when the spec is not available.
func structInputValues(m map[string]any, workflow string) []InputValue {
	fields, ok := structFields(workflow)
	if !ok {
		fields = make([]inputField, 0, len(m))
		for name := range m {
			fields = append(fields, inputField{name: name})
		}
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].name < fields[j].name })
	out := make([]InputValue, 0, len(fields))
	for _, f := range fields {
		if v, ok := m[f.name]; ok {
			out = append(out, InputValue{Name: f.name, Value: fmt.Sprint(v)})
		} else {
			out = append(out, InputValue{Name: f.name, Value: zeroOf(f.typ)})
		}
	}
	return out
}

// structFields is the fields of the workflow's struct input (the spec's
// inputs — a struct input has several fields; a simple input has one).
func structFields(workflow string) ([]inputField, bool) {
	spec, ok := registry.SpecOf(workflow)
	if !ok || len(spec.Inputs) <= 1 {
		return nil, false
	}
	out := make([]inputField, 0, len(spec.Inputs))
	for _, p := range spec.Inputs {
		out = append(out, inputField{name: p.Name, typ: p.Type})
	}
	return out, true
}

// zeroOf is the Go zero value of the field's type (the run received the
// zero when the dispatch did not post the field): "" for string, "0" for
// int (the parser supports string and int inputs).
func zeroOf(typ string) string {
	if typ == "string" {
		return ""
	}
	return "0"
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
