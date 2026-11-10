package graph

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
)

// pathsFromSources parses the given Go source files and builds the maps:
// workflow name → all possible paths, and workflow name → parameter spec
// (input + step defaults). The marker is the per-file DBOS registration
// RegisterWorkflow(ctx, Func, WithWorkflowName("name"), ...) — matched by
// function name, so any import alias of the dbos package is found. A
// RunAsStep call adds a step to every open path; an if forks the paths
// (taken branch vs not-taken branch).
func pathsFromSources(files map[string][]byte) (map[string]*WorkflowGraph, map[string]Spec, map[string]string, error) {
	fset := token.NewFileSet()
	funcs := map[string]*ast.FuncDecl{}
	regs := map[string]string{} // registered name → workflow func name
	funcPkg := map[string]string{}
	funcFile := map[string]string{} // workflow func name → the file that declared it
	pkgConstants := map[string]map[string]string{}
	pkgTypes := map[string]map[string][]Param{}
	for name, data := range files {
		file, err := parser.ParseFile(fset, name, data, 0)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("parse %s: %w", name, err)
		}
		collectFuncs(file, name, funcs, funcPkg, funcFile)
		collectRegistrations(file, regs)
		collectConstants(file, pkgConstants)
		collectTypes(file, pkgTypes)
	}
	filesByFunc := make(map[string]string, len(regs))
	for name, fnName := range regs {
		if f := funcFile[fnName]; f != "" {
			filesByFunc[name] = f
		}
	}
	return pathsFromRegistrations(regs, funcs),
		specsFromRegistrations(regs, funcs, funcPkg, pkgConstants, pkgTypes), filesByFunc, nil
}

// collectFuncs records the file's function declarations (the workflow
// func bodies are walked later, by possiblePaths), the package each
// func is declared in, and the file path that declared it (the key of
// the files map the parse walked).
func collectFuncs(file *ast.File, fileKey string, funcs map[string]*ast.FuncDecl, funcPkg map[string]string, funcFile map[string]string) {
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			funcs[fd.Name.Name] = fd
			funcPkg[fd.Name.Name] = file.Name.Name
			funcFile[fd.Name.Name] = fileKey
		}
	}
}

// collectConstants records the package's constant values (name → literal
// value): the input defaults (const default<NamePascal> = "...").
func collectConstants(file *ast.File, consts map[string]map[string]string) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		if consts[file.Name.Name] == nil {
			consts[file.Name.Name] = map[string]string{}
		}
		for _, spec := range gd.Specs {
			addConstant(file, spec, consts)
		}
	}
}

// addConstant records the literal value of one const spec (a string:
// without the quotes; a number: as written) — non-literal constants are
// skipped.
func addConstant(file *ast.File, spec ast.Spec, consts map[string]map[string]string) {
	vs, ok := spec.(*ast.ValueSpec)
	if !ok || len(vs.Names) == 0 || len(vs.Values) == 0 {
		return
	}
	lit, ok := vs.Values[0].(*ast.BasicLit)
	if !ok {
		return
	}
	value := lit.Value
	if lit.Kind == token.STRING { // a string literal: without the quotes
		value, _ = strconv.Unquote(lit.Value)
	}
	for _, name := range vs.Names {
		consts[file.Name.Name][name.Name] = value
	}
}

// collectTypes records the package's struct types (type name → the
// fields, in source order): the fields of a struct input are the
// workflow's inputs.
func collectTypes(file *ast.File, types map[string]map[string][]Param) {
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.TYPE {
			continue
		}
		for _, spec := range gd.Specs {
			if ts, ok := spec.(*ast.TypeSpec); ok {
				addType(file, ts, types)
			}
		}
	}
}

func addType(file *ast.File, ts *ast.TypeSpec, types map[string]map[string][]Param) {
	if types[file.Name.Name] == nil {
		types[file.Name.Name] = map[string][]Param{}
	}
	types[file.Name.Name][ts.Name.Name] = structFields(ts.Type)
}

// structFields is the fields of a struct type (name + type, in source
// order) — an embedded field has no name and is not an input; a non-struct
// type has no fields.
func structFields(expr ast.Expr) []Param {
	st, ok := expr.(*ast.StructType)
	if !ok {
		return nil
	}
	var fields []Param
	for _, f := range st.Fields.List {
		for _, name := range f.Names {
			fields = append(fields, Param{Name: name.Name, Type: typeName(f.Type)})
		}
	}
	return fields
}

// collectRegistrations records the per-file DBOS registrations:
// RegisterWorkflow(ctx, Func, WithWorkflowName("name"), ...) — matched by
// function name, so any import alias of the dbos package is found.
func collectRegistrations(file *ast.File, regs map[string]string) {
	ast.Inspect(file, func(n ast.Node) bool {
		call, isCall := n.(*ast.CallExpr)
		if !isCall || len(call.Args) < 2 || calleeName(call.Fun) != "RegisterWorkflow" {
			return true
		}
		fn, ok := call.Args[1].(*ast.Ident)
		if !ok {
			return true
		}
		regName, ok := optionName(call, "WithWorkflowName")
		if !ok {
			return true
		}
		regs[regName] = fn.Name
		return true
	})
}

// pathsFromRegistrations builds the map: registered name → the stored block
// model (workflow funcs not defined in this package's source are skipped).
func pathsFromRegistrations(regs map[string]string, funcs map[string]*ast.FuncDecl) map[string]*WorkflowGraph {
	out := make(map[string]*WorkflowGraph, len(regs))
	for name, fnName := range regs {
		fd := funcs[fnName]
		if fd == nil || fd.Body == nil {
			continue // workflow func not defined in this package's source
		}
		out[name] = buildGraph(fd.Body)
	}
	return out
}
