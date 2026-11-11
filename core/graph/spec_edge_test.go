package graph

import (
	"go/ast"
	"testing"
)

// The spec edge cases: a registration whose workflow func is not defined in
// the walked source is skipped; inputsOf is nil for a func with no
// parameters; Pascal drops empty parts; an unnamed field has no param name;
// a non-name type expr has no type name; a RunAsStep without a step arg
// has no default.

// A registration that references a workflow func not defined in the walked
// source is skipped: the spec map has no entry for it.
func TestSpecsSkipAnUndefinedWorkflowFunc(t *testing.T) {
	specs, err := parseSpecs(map[string][]byte{
		"wf.go": []byte(`package wf
func init() {
	dbos.RegisterWorkflow(ctx, Elsewhere, dbos.WithWorkflowName("x"))
}
`),
	})
	if err != nil {
		t.Fatalf("parseSpecs: %v", err)
	}
	if _, ok := specs["x"]; ok {
		t.Errorf("specs: got an entry for x, want none (the func is undefined)")
	}
	if len(specs) != 0 {
		t.Errorf("specs: got %v, want empty", specs)
	}
}

// inputsOf is nil for a func without parameters (the ctx-only workflow).
func TestInputsOfNilForAParamlessFunc(t *testing.T) {
	fd := &ast.FuncDecl{Type: &ast.FuncType{}}
	if in := inputsOf(fd, "wf", nil, nil); in != nil {
		t.Errorf("inputsOf(paramless): got %v, want nil", in)
	}
}

// Pascal maps the name to the default constant's suffix, dropping empty
// parts (double underscores, a leading underscore).
func TestPascalDropsEmptyParts(t *testing.T) {
	cases := map[string]string{
		"msg":        "Msg",
		"start_date": "StartDate",
		"a__b":       "AB",
		"_x":         "X",
		"":           "",
	}
	for in, want := range cases {
		if got := Pascal(in); got != want {
			t.Errorf("Pascal(%q): got %q, want %q", in, got, want)
		}
	}
}

// An unnamed field (an embedded field) has no param name.
func TestParamNameUnnamedField(t *testing.T) {
	f := &ast.Field{Type: &ast.Ident{Name: "string"}}
	if got := paramName(f); got != "" {
		t.Errorf("paramName(unnamed): got %q, want empty", got)
	}
}

// A type expr that is neither an ident nor a selector (a pointer) has no
// type name.
func TestTypeNameNonNameExpr(t *testing.T) {
	ptr := &ast.StarExpr{X: &ast.Ident{Name: "T"}}
	if got := typeName(ptr); got != "" {
		t.Errorf("typeName(pointer): got %q, want empty", got)
	}
}

// addConstant skips a non-ValueSpec decl spec (the const specs are all
// ValueSpecs, so the guard is pinned by a direct call with one).
func TestAddConstantSkipsANonValueSpec(t *testing.T) {
	consts := map[string]map[string]string{"wf": {}}
	file := &ast.File{Name: &ast.Ident{Name: "wf"}}
	addConstant(file, &ast.TypeSpec{Name: &ast.Ident{Name: "T"}}, consts)
	if len(consts["wf"]) != 0 {
		t.Errorf("addConstant(non-ValueSpec): got %v, want nothing recorded", consts["wf"])
	}
}

// A RunAsStep without a step arg has no default (no literal to read).
func TestLiteralArgWithoutAStepArg(t *testing.T) {
	call := runCall(t, "_ = dbos.RunAsStep(ctx)")
	if got := literalArg(call); got != "" {
		t.Errorf("literalArg(no step arg): got %q, want empty", got)
	}
}
