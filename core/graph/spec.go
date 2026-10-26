package graph

import (
	"errors"
	"fmt"
	"go/ast"
	"strconv"
	"strings"
)

// Spec is the parameter spec of one workflow, parsed from its source
// (phase 5, BOOT): the workflow's inputs (each with its default value,
// the package constant default<NamePascal>) and the steps' default values.
type Spec struct {
	// Inputs is the workflow's input, in source order: the fields of the
	// struct input, or the single simple parameter (string, int).
	Inputs []Param
	// Steps is the steps in source order, each with its default value.
	Steps []StepSpec
}

// ErrInvalidInput is returned by Spec.Sanitize when the form values do not
// sanitize against the spec (an unknown field, a value that does not
// convert to the input's type). The web raises it as a 400.
var ErrInvalidInput = errors.New("graph: invalid input")

// Param is one input of a workflow.
type Param struct {
	Name    string
	Type    string
	Default string // the default value (const default<NamePascal>), "" when absent
}

// Sanitize maps the raw form values onto the spec's clean input (phase 5,
// EXECUTE): no extra fields, each value is trimmed and converts to the
// input's type, the missing values fall back to the inputs' zero values.
// The payload is the bare value (a single simple input) or the map of
// field → value (a struct input). A nil values map is the zero payload
// (a dispatch without a form).
func (s Spec) Sanitize(values map[string]string) (any, error) {
	if values == nil {
		return nil, nil
	}
	if len(s.Inputs) == 0 {
		if len(values) != 0 {
			return nil, fmt.Errorf("%w: spec has no input", ErrInvalidInput)
		}
		return nil, nil
	}
	if err := s.checkFields(values); err != nil {
		return nil, err
	}
	if len(s.Inputs) == 1 {
		// The form value is trimmed before the conversion to the type.
		return s.Inputs[0].convert(strings.TrimSpace(values[s.Inputs[0].Name]))
	}
	payload := make(map[string]any, len(s.Inputs))
	for _, in := range s.Inputs {
		value, err := in.convert(strings.TrimSpace(values[in.Name]))
		if err != nil {
			return nil, err
		}
		payload[in.Name] = value
	}
	return payload, nil
}

// checkFields rejects a form field that is not one of the spec's inputs.
func (s Spec) checkFields(values map[string]string) error {
	known := make(map[string]struct{}, len(s.Inputs))
	for _, in := range s.Inputs {
		known[in.Name] = struct{}{}
	}
	for field := range values {
		if _, ok := known[field]; !ok {
			return fmt.Errorf("%w: unknown field %q", ErrInvalidInput, field)
		}
	}
	return nil
}

// convert maps one form value to the input's type (the spec only
// understands string and int).
func (p Param) convert(value string) (any, error) {
	switch p.Type {
	case "string":
		return value, nil
	case "int":
		if value == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not an int", ErrInvalidInput, value)
		}
		return n, nil
	default:
		return nil, fmt.Errorf("%w: unsupported input type %q", ErrInvalidInput, p.Type)
	}
}

// StepSpec is one step: its name and its default value (the literal the
// source passes to the step call — a step that takes the workflow's input
// has no default).
type StepSpec struct {
	Name    string
	Default string
}

// specsFromRegistrations builds the map: registered name → spec (inputs +
// step defaults), the same registrations the paths are built from.
func specsFromRegistrations(regs map[string]string, funcs map[string]*ast.FuncDecl,
	funcPkg map[string]string, pkgConstants map[string]map[string]string,
	pkgTypes map[string]map[string][]Param) map[string]Spec {
	out := make(map[string]Spec, len(regs))
	for name, fnName := range regs {
		fd := funcs[fnName]
		if fd == nil || fd.Body == nil {
			continue // workflow func not defined in this package's source
		}
		spec := Spec{
			Inputs: inputsOf(fd, funcPkg[fnName], pkgConstants, pkgTypes),
			Steps:  stepSpecs(fd.Body),
		}
		out[name] = spec
	}
	return out
}

// inputsOf is the workflow's inputs: the fields of the struct input (the
// type of the first non-context parameter, declared in the package), or
// the single simple parameter. Each input carries its default (the
// package constant default<NamePascal>).
func inputsOf(fd *ast.FuncDecl, pkg string,
	pkgConstants map[string]map[string]string,
	pkgTypes map[string]map[string][]Param) []Param {
	if fd.Type == nil || fd.Type.Params == nil {
		return nil
	}
	for _, f := range fd.Type.Params.List {
		if in := paramInput(f, pkgConstants[pkg], pkgTypes[pkg]); in != nil {
			return in
		}
	}
	return nil
}

// paramInput is the input of one parameter of the workflow: the struct's
// fields (a struct input), or the single simple parameter. Context and the
// blank parameter (the Go SDK's required input argument) are not inputs.
func paramInput(f *ast.Field, consts map[string]string,
	types map[string][]Param) []Param {
	if typeName(f.Type) == "Context" || paramName(f) == "_" {
		return nil
	}
	name := paramName(f)
	if fields, ok := types[typeName(f.Type)]; ok {
		out := make([]Param, len(fields))
		for i, field := range fields {
			out[i] = Param{Name: field.Name, Type: field.Type,
				Default: consts["default"+Pascal(field.Name)]}
		}
		return out
	}
	return []Param{{Name: name, Type: typeName(f.Type), Default: consts["default"+Pascal(name)]}}
}

// Pascal maps the parameter name to the default constant's suffix: msg →
// Msg, start_date → StartDate.
func Pascal(s string) string {
	parts := strings.Split(s, "_")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" {
			continue
		}
		out = append(out, strings.ToUpper(part[:1])+part[1:])
	}
	return strings.Join(out, "")
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
