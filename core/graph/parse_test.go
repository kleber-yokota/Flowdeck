package graph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"
)

// given a source with a RegisterWorkflow registration,
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
	byName, _, _, err := pathsFromSources(map[string][]byte{"workflows.go": []byte(src)})
	if err != nil {
		t.Errorf("pathsFromSources: %v", err)
		return
	}
	if len(byName) != 1 {
		t.Errorf("pathsFromSources: found %d workflows, want 1: %v", len(byName), byName)
		return
	}
	w := byName["branchy"]
	if w == nil {
		t.Errorf("ByName: missing the branchy graph: %v", byName)
		return
	}
	if !equalPaths(w.Paths, [][]string{
		{"one", "two", "three", "five"},
		{"one", "two", "four", "five"},
	}) {
		t.Errorf("Paths(branchy): got %v", w.Paths)
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
	byName, _, _, err := pathsFromSources(map[string][]byte{"workflows.go": []byte(src)})
	if err != nil {
		t.Errorf("pathsFromSources: %v", err)
		return
	}
	if len(byName) != 1 {
		t.Errorf("pathsFromSources: found %d workflows, want 1: %v", len(byName), byName)
		return
	}
	w := byName["aliased"]
	if w == nil {
		t.Errorf("ByName: missing the alias graph: %v", byName)
		return
	}
	if !equalPaths(w.Paths, [][]string{
		{"one", "two", "three"},
		{"one", "three"},
	}) {
		t.Errorf("Paths(aliased): got %v", w.Paths)
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

// The Files map: each registered workflow name → the file that declared
// its workflow func — the subpackage key keeps its dir/file form, the
// flat key stays bare (the boot map's breadcrumb source).
func TestPathsFromSourcesFiles(t *testing.T) {
	const sub = `package pipeline

import dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Pipeline, dbos.WithWorkflowName("pipeline"))
}

func Pipeline(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, oneStep(), dbos.WithStepName("one"))
	return nil
}
`
	const flat = `package workflows

import dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Flat, dbos.WithWorkflowName("flat"))
}

func Flat(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, oneStep(), dbos.WithStepName("one"))
	return nil
}
`
	files := map[string][]byte{
		"pipeline/pipeline.go": []byte(sub),
		"workflows.go":         []byte(flat),
	}
	_, _, fs, err := pathsFromSources(files)
	if err != nil {
		t.Errorf("pathsFromSources: %v", err)
		return
	}
	if fs["pipeline"] != "pipeline/pipeline.go" {
		t.Errorf("Files[pipeline]: got %q, want pipeline/pipeline.go", fs["pipeline"])
	}
	if fs["flat"] != "workflows.go" {
		t.Errorf("Files[flat]: got %q, want workflows.go", fs["flat"])
	}
}
