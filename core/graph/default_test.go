package graph

import (
	"testing"
)

// Phase 5, BOOT: the parser extracts each workflow's inputs (one per
// parameter of the source) and their default values — the input default
// is the package-level constant default<NamePascal> (msg → defaultMsg,
// start_date → defaultStartDate).

func parseSpecs(files map[string][]byte) (map[string]Spec, error) {
	_, specs, err := pathsFromSources(files)
	return specs, err
}

// The default of a simple input is the package constant defaultMsg.
func TestInputDefaultFromPackageConstant(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"wf.go": []byte(`package wf

import dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

const defaultMsg = "FlowDeck"

func init() {
	dbos.RegisterWorkflow(ctx, Hello, dbos.WithWorkflowName("hello"))
}

func Hello(ctx dbos.Context, msg string) error {
	return nil
}
`),
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	spec, ok := specs["hello"]
	if !ok {
		t.Fatal("spec: workflow hello not found")
	}
	if len(spec.Inputs) != 1 {
		t.Fatalf("inputs: got %v, want 1", spec.Inputs)
	}
	in := spec.Inputs[0]
	if in.Name != "msg" || in.Type != "string" || in.Default != "FlowDeck" {
		t.Fatalf("input: got %+v, want {msg string FlowDeck}", in)
	}
}

// A snake_case input is Pascal-cased in the constant name: start_date →
// defaultStartDate.
func TestInputDefaultSnakeCaseParam(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"wf.go": []byte(`package wf

const defaultStartDate = "2026-01-01"

func init() {
	dbos.RegisterWorkflow(ctx, Schedule, dbos.WithWorkflowName("schedule"))
}

func Schedule(ctx dbos.Context, start_date string) error {
	return nil
}
`),
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	spec, ok := specs["schedule"]
	if !ok {
		t.Fatal("spec: workflow schedule not found")
	}
	if len(spec.Inputs) != 1 {
		t.Fatalf("inputs: got %v, want 1", spec.Inputs)
	}
	if in := spec.Inputs[0]; in.Name != "start_date" || in.Type != "string" || in.Default != "2026-01-01" {
		t.Fatalf("input: got %+v, want {start_date string 2026-01-01}", in)
	}
}

// A workflow without the constant has no default.
func TestInputDefaultAbsent(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"wf.go": []byte(`package wf

func init() {
	dbos.RegisterWorkflow(ctx, Hello, dbos.WithWorkflowName("hello"))
}

func Hello(ctx dbos.Context, msg string) error {
	return nil
}
`),
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	spec, ok := specs["hello"]
	if !ok {
		t.Fatal("spec: workflow hello not found")
	}
	if len(spec.Inputs) != 1 {
		t.Fatalf("inputs: got %v, want 1", spec.Inputs)
	}
	if in := spec.Inputs[0]; in.Name != "msg" || in.Default != "" {
		t.Fatalf("input: got %+v, want {msg string }", in)
	}
}

// Two packages with the same input name have their own defaults.
func TestInputDefaultPerPackage(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"a/wf.go": []byte(`package a

const defaultMsg = "A"

func init() {
	dbos.RegisterWorkflow(ctx, Hello, dbos.WithWorkflowName("hello-a"))
}

func Hello(ctx dbos.Context, msg string) error {
	return nil
}
`),
		"b/wf.go": []byte(`package b

const defaultMsg = "B"

func init() {
	dbos.RegisterWorkflow(ctx, Hello, dbos.WithWorkflowName("hello-b"))
}

func Hello(ctx dbos.Context, msg string) error {
	return nil
}
`),
	})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	// The two registrations (one per package) map to the two specs.
	gotA, gotB := "A", "B"
	for _, spec := range specs {
		if in := spec.Inputs[0]; in.Default == "A" {
			gotA = in.Default
		}
		if in := spec.Inputs[0]; in.Default == "B" {
			gotB = in.Default
		}
	}
	if gotA != "A" || gotB != "B" {
		t.Fatalf("per-package defaults: got %q %q", gotA, gotB)
	}
}

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
		t.Fatalf("parse: %v", err)
	}
	spec, ok := specs["schedule"]
	if !ok {
		t.Fatal("spec: workflow schedule not found")
	}
	if len(spec.Inputs) != 2 {
		t.Fatalf("inputs: got %v, want 2", spec.Inputs)
	}
	if in := spec.Inputs[0]; in.Name != "Name" || in.Type != "string" || in.Default != "FlowDeck" {
		t.Fatalf("input 0: got %+v, want {Name string FlowDeck}", in)
	}
	if in := spec.Inputs[1]; in.Name != "StartDate" || in.Type != "string" || in.Default != "2026-01-01" {
		t.Fatalf("input 1: got %+v, want {StartDate string 2026-01-01}", in)
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
		t.Fatalf("parse: %v", err)
	}
	spec, ok := specs["ping"]
	if !ok {
		t.Fatal("spec: workflow ping not found")
	}
	if len(spec.Inputs) != 0 {
		t.Fatalf("inputs: got %v, want none", spec.Inputs)
	}
}
