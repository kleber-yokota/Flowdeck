package graph

import (
	"go/ast"
	"testing"
)

// The parser guards: a non-literal constant is not recorded (the input
// default stays empty); a non-struct type has no fields; a registration
// without a func arg (not an ident) or without the workflow-name option is
// not a registration; a registration whose workflow func is not defined in
// the walked source builds no graph.

// A non-literal constant is not recorded: the input's default stays empty.
func TestSpecDefaultFromANonLiteralConstant(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"wf.go": []byte(`package wf
const defaultMsg = 1 + 2
func init() {
	dbos.RegisterWorkflow(ctx, Wf, dbos.WithWorkflowName("wf"))
}
func Wf(ctx dbos.Context, msg string) error {
	return nil
}
`),
	})
	if err != nil {
		t.Fatalf("parseSpecs: %v", err)
	}
	spec, ok := specs["wf"]
	if !ok {
		t.Fatal("specs: workflow wf not found")
	}
	if len(spec.Inputs) != 1 || spec.Inputs[0].Default != "" {
		t.Errorf("inputs: got %+v, want the msg input with no default", spec.Inputs)
	}
}

// A non-struct type (a string) has no fields: the param is the single
// simple input, not the struct's fields.
func TestStructFieldsNonStruct(t *testing.T) {
	if got := structFields(&ast.Ident{Name: "string"}); got != nil {
		t.Errorf("structFields(string): got %v, want nil", got)
	}
}

// A registration whose func arg is not an ident is not a registration: no
// graph and no spec are built for it.
func TestCollectRegistrationsSkipsANonIdentFuncArg(t *testing.T) {
	byName, specs, _, err := pathsFromSources(map[string][]byte{
		"wf.go": []byte(`package wf
func init() {
	dbos.RegisterWorkflow(ctx, 42, dbos.WithWorkflowName("x"))
}
`),
	})
	if err != nil {
		t.Fatalf("pathsFromSources: %v", err)
	}
	if len(byName) != 0 || len(specs) != 0 {
		t.Errorf("non-ident func arg: got %v / %v, want none", byName, specs)
	}
}

// A registration without the workflow-name option is not a registration:
// no graph and no spec are built for it.
func TestCollectRegistrationsSkipsANUnnamedRegistration(t *testing.T) {
	byName, specs, _, err := pathsFromSources(map[string][]byte{
		"wf.go": []byte(`package wf
func init() {
	dbos.RegisterWorkflow(ctx, Wf)
}
func Wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	return nil
}
`),
	})
	if err != nil {
		t.Fatalf("pathsFromSources: %v", err)
	}
	if len(byName) != 0 || len(specs) != 0 {
		t.Errorf("unnamed registration: got %v / %v, want none", byName, specs)
	}
}

// A registration whose workflow func is not defined in the walked source
// builds no graph: the ByName map has no entry for it.
func TestPathsSkipAnUndefinedWorkflowFunc(t *testing.T) {
	byName, _, _, err := pathsFromSources(map[string][]byte{
		"wf.go": []byte(`package wf
func init() {
	dbos.RegisterWorkflow(ctx, Elsewhere, dbos.WithWorkflowName("x"))
}
`),
	})
	if err != nil {
		t.Fatalf("pathsFromSources: %v", err)
	}
	if len(byName) != 0 {
		t.Errorf("ByName: got %v, want empty", byName)
	}
}
