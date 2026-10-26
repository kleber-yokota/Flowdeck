package graph

import (
	"go/ast"
	"strconv"
)

// Spec is the parameter spec of one workflow, parsed from its source
// (phase 5, BOOT): the workflow's input and the steps' default values.
type Spec struct {
	// Input is the workflow's input parameter (name + type).
	Input Param
	// Steps is the steps in source order, each with its default value.
	Steps []StepSpec
}

// Param is one parameter of a workflow's input.
type Param struct {
	Name string
	Type string
}

// StepSpec is one step: its name and its default value (the literal the
// source passes to the step call — a step that takes the workflow's input
// has no default).
type StepSpec struct {
	Name    string
	Default string
}

// specsFromRegistrations builds the map: registered name → spec (input +
// step defaults), the same registrations the paths are built from.
func specsFromRegistrations(regs map[string]string, funcs map[string]*ast.FuncDecl) map[string]Spec {
	out := make(map[string]Spec, len(regs))
	for name, fnName := range regs {
		fd := funcs[fnName]
		if fd == nil || fd.Body == nil {
			continue // workflow func not defined in this package's source
		}
		spec := Spec{Input: inputParam(fd)}
		spec.Steps = stepSpecs(fd.Body)
		out[name] = spec
	}
	return out
}

// inputParam is the workflow's input: the first parameter that is not a
// context (the DBOS context comes first).
func inputParam(fd *ast.FuncDecl) Param {
	if fd.Type == nil || fd.Type.Params == nil {
		return Param{}
	}
	for _, f := range fd.Type.Params.List {
		if typeName(f.Type) == "Context" {
			continue
		}
		return Param{Name: paramName(f), Type: typeName(f.Type)}
	}
	return Param{}
}

func paramName(field *ast.Field) string {
	if len(field.Names) == 0 {
		return ""
	}
	return field.Names[0].Name
}

func typeName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	}
	return ""
}

// stepSpecs returns the steps of the body (name + default value), in
// source order: the default is the literal the source passes to the step
// call — a step that takes the workflow's input (a variable, a closure)
// has no default.
func stepSpecs(body *ast.BlockStmt) []StepSpec {
	var specs []StepSpec
	ast.Inspect(body, func(n ast.Node) bool {
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		call, isCall := n.(*ast.CallExpr)
		if !isCall || calleeName(call.Fun) != "RunAsStep" {
			return true
		}
		specs = append(specs, StepSpec{Name: runAsStepName(call), Default: literalArg(call)})
		return false
	})
	return specs
}

// literalArg is the default value of a step call: the literal the source
// passes to the step function (EchoStep("fetch") — the first argument of
// the call, when it is a literal). A step that takes the workflow's input
// (a variable, a closure) has no default.
func literalArg(call *ast.CallExpr) string {
	if len(call.Args) < 2 {
		return ""
	}
	arg := call.Args[1]
	if inner, isCall := arg.(*ast.CallExpr); isCall && len(inner.Args) > 0 {
		arg = inner.Args[0]
	}
	lit, ok := arg.(*ast.BasicLit)
	if !ok {
		return ""
	}
	value, _ := strconv.Unquote(lit.Value)
	return value
}
