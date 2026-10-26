package graph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// Sequential RunAsStep calls form a single linear path: all the steps,
// in source order.
func TestPossiblePathsLinearSteps(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("one"))
	_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("two"))
	_ = dbos.RunAsStep(ctx, cStep(), dbos.WithStepName("three"))
	_ = dbos.RunAsStep(ctx, dStep(), dbos.WithStepName("four"))
	_ = dbos.RunAsStep(ctx, eStep(), dbos.WithStepName("five"))
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	got := possiblePaths(body)
	if !equalPaths(got, [][]string{{"one", "two", "three", "four", "five"}}) {
		t.Fatalf("possiblePaths: got %v", got)
	}
}

// An if forks the possible paths: the taken branch (if-body steps) and the
// not-taken one (else-body steps).
func TestPossiblePathsBranchOnIf(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	if ok {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("b"))
	} else {
		_ = dbos.RunAsStep(ctx, cStep(), dbos.WithStepName("c"))
	}
	_ = dbos.RunAsStep(ctx, dStep(), dbos.WithStepName("d"))
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	got := possiblePaths(body)
	if !equalPaths(got, [][]string{{"a", "b", "d"}, {"a", "c", "d"}}) {
		t.Fatalf("possiblePaths: got %v", got)
	}
}

// An if without else also leaves the path without the if steps (the branch
// may not be taken).
func TestPossiblePathsBranchWithoutElse(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	if ok {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("b"))
	}
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	got := possiblePaths(body)
	if !equalPaths(got, [][]string{{"a", "b"}, {"a"}}) {
		t.Fatalf("possiblePaths: got %v", got)
	}
}

// Phase 4, STEP 1: given a source with a RegisterWorkflow registration,
// the parser finds the workflow and generates its full graph — the steps
// in order, forked by the if/else.
func TestPathsFromSourcesFindsRegistrationAndGraph(t *testing.T) {
	const src = `package core

import dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

func init() {
	register("branchy",
		func(ctx dbos.Context) {
			dbos.RegisterWorkflow(ctx, Branchy,
				dbos.WithWorkflowName("branchy"),
				dbos.WithMaxRecoveryAttempts(3),
			)
		},
		Branchy,
	)
}

func Branchy(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, oneStep(), dbos.WithStepName("one"))
	_ = dbos.RunAsStep(ctx, twoStep(), dbos.WithStepName("two"))
	if ok {
		_ = dbos.RunAsStep(ctx, threeStep(), dbos.WithStepName("three"))
	} else {
		_ = dbos.RunAsStep(ctx, fourStep(), dbos.WithStepName("four"))
	}
	_ = dbos.RunAsStep(ctx, fiveStep(), dbos.WithStepName("five"))
	return nil
}
`
	byName, _, err := pathsFromSources(map[string][]byte{"workflows.go": []byte(src)})
	if err != nil {
		t.Fatalf("pathsFromSources: %v", err)
	}
	if len(byName) != 1 {
		t.Fatalf("pathsFromSources: found %d workflows, want 1: %v", len(byName), byName)
	}
	if !equalPaths(byName["branchy"], [][]string{
		{"one", "two", "three", "five"},
		{"one", "two", "four", "five"},
	}) {
		t.Fatalf("Paths(branchy): got %v", byName["branchy"])
	}
}

// The dbos package can be imported under any alias (dbostx, tx, ...) —
// the parser matches the callee names, so the registration, the workflow
// name and the steps are still found.
func TestPathsFromSourcesImportAlias(t *testing.T) {
	const src = `package core

import dbostx "github.com/dbos-inc/dbos-transact-golang/dbos"

func init() {
	register("aliased",
		func(ctx dbostx.Context) {
			dbostx.RegisterWorkflow(ctx, Aliased,
				dbostx.WithWorkflowName("aliased"),
				dbostx.WithMaxRecoveryAttempts(3),
			)
		},
		Aliased,
	)
}

func Aliased(ctx dbostx.Context) error {
	_ = dbostx.RunAsStep(ctx, oneStep(), dbostx.WithStepName("one"))
	if ok {
		_ = dbostx.RunAsStep(ctx, twoStep(), dbostx.WithStepName("two"))
	}
	_ = dbostx.RunAsStep(ctx, threeStep(), dbostx.WithStepName("three"))
	return nil
}
`
	byName, _, err := pathsFromSources(map[string][]byte{"workflows.go": []byte(src)})
	if err != nil {
		t.Fatalf("pathsFromSources: %v", err)
	}
	if len(byName) != 1 {
		t.Fatalf("pathsFromSources: found %d workflows, want 1: %v", len(byName), byName)
	}
	if !equalPaths(byName["aliased"], [][]string{
		{"one", "two", "three"},
		{"one", "three"},
	}) {
		t.Fatalf("Paths(aliased): got %v", byName["aliased"])
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
	_, specs, err := pathsFromSources(map[string][]byte{"workflows.go": []byte(src)})
	if err != nil {
		t.Fatalf("pathsFromSources: %v", err)
	}
	spec, ok := specs["greet"]
	if !ok {
		t.Fatalf("specs: missing workflow greet: %v", specs)
	}
	if len(spec.Inputs) != 1 {
		t.Fatalf("spec inputs: got %v, want 1", spec.Inputs)
	}
	if in := spec.Inputs[0]; in.Name != "msg" || in.Type != "string" {
		t.Fatalf("spec input: got %+v, want {msg string}", in)
	}
	if len(spec.Steps) != 2 {
		t.Fatalf("spec steps: got %v, want 2", spec.Steps)
	}
	if spec.Steps[0].Name != "greet" || spec.Steps[0].Default != "" {
		t.Fatalf("step greet: got %+v — a step that takes the workflow input has no default", spec.Steps[0])
	}
	if spec.Steps[1].Name != "fetch" || spec.Steps[1].Default != "fetch" {
		t.Fatalf("step fetch: got %+v", spec.Steps[1])
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
	_, specs, err := pathsFromSources(map[string][]byte{"workflows.go": []byte(src)})
	if err != nil {
		t.Fatalf("pathsFromSources: %v", err)
	}
	spec, ok := specs["count"]
	if !ok {
		t.Fatalf("specs: missing workflow count: %v", specs)
	}
	if len(spec.Inputs) != 1 {
		t.Fatalf("spec inputs: got %v, want 1", spec.Inputs)
	}
	if in := spec.Inputs[0]; in.Name != "n" || in.Type != "int" {
		t.Fatalf("spec input: got %+v, want {n int}", in)
	}
}

// parseSnippet parses a workflow-func snippet and returns its body.
func parseSnippet(src string) (*ast.BlockStmt, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "snippet.go", "package p\n"+src, 0)
	if err != nil {
		return nil, err
	}
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok || fd.Name.Name != "wf" {
			continue
		}
		return fd.Body, nil
	}
	return nil, nil
}

func equalPaths(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !slices.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
