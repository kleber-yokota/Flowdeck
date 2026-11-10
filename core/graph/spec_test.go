package graph

import (
	"testing"
)

// A struct input: the form has one item per field (the fields are the
// inputs), each with its own default constant.
func TestSpecStructInput(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"wf.go": []byte(`package wf

const defaultName = "FlowDeck"
const defaultStartDate = "2026-01-01"

type ScheduleArgs struct {
	Name      string
	StartDate string
}

func init() {
	dbos.RegisterWorkflow(ctx, Schedule, dbos.WithWorkflowName("schedule"))
}

func Schedule(ctx dbos.Context, args ScheduleArgs) error {
	return nil
}
`),
	})
	if err != nil {
		t.Errorf("parse: %v", err)
		return
	}
	spec, ok := specs["schedule"]
	if !ok {
		t.Error("spec: workflow schedule not found")
		return
	}
	if len(spec.Inputs) != 2 {
		t.Errorf("inputs: got %v, want 2", spec.Inputs)
		return
	}
	if in := spec.Inputs[0]; in.Name != "Name" || in.Type != "string" || in.Default != "FlowDeck" {
		t.Errorf("input 0: got %+v, want {Name string FlowDeck}", in)
	}
	if in := spec.Inputs[1]; in.Name != "StartDate" || in.Type != "string" || in.Default != "2026-01-01" {
		t.Errorf("input 1: got %+v, want {StartDate string 2026-01-01}", in)
	}
}

// The blank parameter (the Go SDK requires the workflow's input argument;
// blank means the workflow has no input) is not an input — the web renders
// no form.
func TestSpecBlankParamIsNotAnInput(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"wf.go": []byte(`package wf

func init() {
	dbos.RegisterWorkflow(ctx, Pause, dbos.WithWorkflowName("pause"))
}

func Pause(ctx dbos.Context, _ string) error {
	return nil
}
`),
	})
	if err != nil {
		t.Errorf("parse: %v", err)
		return
	}
	spec, ok := specs["pause"]
	if !ok {
		t.Error("spec: workflow pause not found")
		return
	}
	if len(spec.Inputs) != 0 {
		t.Errorf("inputs: got %v, want none", spec.Inputs)
	}
}

// A workflow without an input parameter has no inputs (the web renders
// no form).
func TestSpecNoInput(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"wf.go": []byte(`package wf

func init() {
	dbos.RegisterWorkflow(ctx, Ping, dbos.WithWorkflowName("ping"))
}

func Ping(ctx dbos.Context) error {
	return nil
}
`),
	})
	if err != nil {
		t.Errorf("parse: %v", err)
		return
	}
	spec, ok := specs["ping"]
	if !ok {
		t.Error("spec: workflow ping not found")
		return
	}
	if len(spec.Inputs) != 0 {
		t.Errorf("inputs: got %v, want none", spec.Inputs)
	}
}

// Phase 5, BOOT: the parser extracts each workflow's input (the name and
// type of the workflow function's first non-context parameter) and the
// steps' default values (the literals the source passes to the step calls —
// a step that takes the workflow's input has no default).
func TestSpecsFromSourcesInputAndStepDefaults(t *testing.T) {
	const src = `package core

import dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

func init() {
	register("greet",
		func(ctx dbos.Context) {
			dbos.RegisterWorkflow(ctx, Greet,
				dbos.WithWorkflowName("greet"),
				dbos.WithMaxRecoveryAttempts(3),
			)
		},
		Greet,
	)
}

func Greet(ctx dbos.Context, msg string) error {
	_ = dbos.RunAsStep(ctx, greetStep(msg), dbos.WithStepName("greet"))
	_ = dbos.RunAsStep(ctx, echoStep("fetch"), dbos.WithStepName("fetch"))
	return nil
}
`
	_, specs, _, err := pathsFromSources(map[string][]byte{"workflows.go": []byte(src)})
	if err != nil {
		t.Errorf("pathsFromSources: %v", err)
		return
	}
	spec, ok := specs["greet"]
	if !ok {
		t.Errorf("specs: missing workflow greet: %v", specs)
		return
	}
	if len(spec.Inputs) != 1 {
		t.Errorf("spec inputs: got %v, want 1", spec.Inputs)
		return
	}
	if in := spec.Inputs[0]; in.Name != "msg" || in.Type != "string" {
		t.Errorf("spec input: got %+v, want {msg string}", in)
	}
	if len(spec.Steps) != 2 {
		t.Errorf("spec steps: got %v, want 2", spec.Steps)
		return
	}
	if spec.Steps[0].Name != "greet" || spec.Steps[0].Default != "" {
		t.Errorf("step greet: got %+v — a step that takes the workflow input has no default", spec.Steps[0])
	}
	if spec.Steps[1].Name != "fetch" || spec.Steps[1].Default != "fetch" {
		t.Errorf("step fetch: got %+v", spec.Steps[1])
	}
}

// The input's type is part of the spec: the sanitization converts the form
// value to it (an int input is not a string).
func TestSpecsFromSourcesIntInput(t *testing.T) {
	const src = `package core

import dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

func init() {
	register("count",
		func(ctx dbos.Context) {
			dbos.RegisterWorkflow(ctx, Count,
				dbos.WithWorkflowName("count"),
				dbos.WithMaxRecoveryAttempts(3),
			)
		},
		Count,
	)
}

func Count(ctx dbos.Context, n int) error {
	_ = dbos.RunAsStep(ctx, countStep(n), dbos.WithStepName("count"))
	return nil
}
`
	_, specs, _, err := pathsFromSources(map[string][]byte{"workflows.go": []byte(src)})
	if err != nil {
		t.Errorf("pathsFromSources: %v", err)
		return
	}
	spec, ok := specs["count"]
	if !ok {
		t.Errorf("specs: missing workflow count: %v", specs)
		return
	}
	if len(spec.Inputs) != 1 {
		t.Errorf("spec inputs: got %v, want 1", spec.Inputs)
		return
	}
	if in := spec.Inputs[0]; in.Name != "n" || in.Type != "int" {
		t.Errorf("spec input: got %+v, want {n int}", in)
	}
}
