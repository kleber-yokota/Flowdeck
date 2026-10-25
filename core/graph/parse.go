package graph

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// pathsFromSources parses the given Go source files and builds the map:
// workflow name → all possible paths. The marker is the per-file DBOS
// registration RegisterWorkflow(ctx, Func, WithWorkflowName("name"), ...) —
// matched by function name, so any import alias of the dbos package is
// found. A RunAsStep call adds a step to every open path; an if forks the
// paths (taken branch vs not-taken branch).
func pathsFromSources(files map[string][]byte) (map[string][][]string, error) {
	fset := token.NewFileSet()
	funcs := map[string]*ast.FuncDecl{}
	regs := map[string]string{} // registered name → workflow func name
	for name, data := range files {
		file, err := parser.ParseFile(fset, name, data, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", name, err)
		}
		collectFuncs(file, funcs)
		collectRegistrations(file, regs)
	}
	return pathsFromRegistrations(regs, funcs), nil
}

// collectFuncs records the file's function declarations — the workflow
// func bodies are walked later, by possiblePaths.
func collectFuncs(file *ast.File, funcs map[string]*ast.FuncDecl) {
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			funcs[fd.Name.Name] = fd
		}
	}
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

// pathsFromRegistrations builds the map: registered name → all possible
// paths (workflow funcs not defined in this package's source are skipped).
func pathsFromRegistrations(regs map[string]string, funcs map[string]*ast.FuncDecl) map[string][][]string {
	out := make(map[string][][]string, len(regs))
	for name, fnName := range regs {
		fd := funcs[fnName]
		if fd == nil || fd.Body == nil {
			continue // workflow func not defined in this package's source
		}
		out[name] = possiblePaths(fd.Body)
	}
	return out
}

// possiblePaths returns every sequence of steps that can run in the
// function: each RunAsStep call adds a step to every open path, each if
// forks the paths. Empty sequences and duplicates are dropped.
func possiblePaths(body *ast.BlockStmt) [][]string {
	// One open path: the empty sequence.
	paths := [][]string{{}}
	walkStmts(body.List, &paths)
	return dedupePaths(paths)
}

func walkStmts(stmts []ast.Stmt, paths *[][]string) {
	for _, stmt := range stmts {
		walkStmt(stmt, paths)
	}
}

func walkStmt(stmt ast.Stmt, paths *[][]string) {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		walkStmts(s.List, paths)
	case *ast.IfStmt:
		// Fork: every open path splits into the taken branch (if-body
		// steps) and the not-taken one (else-body steps, or no extra step
		// when there is no else).
		taken := [][]string{{}}
		walkStmts(s.Body.List, &taken)
		notTaken := [][]string{{}}
		if s.Else != nil {
			walkStmt(s.Else, &notTaken)
		}
		*paths = forkPaths(*paths, taken, notTaken)
	default:
		if steps := stepsIn(stmt); len(steps) > 0 {
			*paths = addStepToAll(*paths, steps...)
		}
	}
}

// forkPaths splits every open path into taken (p + each taken sequence) and
// not-taken (p + each not-taken sequence). A branch with no RunAsStep is
// the empty sequence (the path keeps going unchanged).
func forkPaths(base, taken, notTaken [][]string) [][]string {
	out := make([][]string, 0, len(base)*2)
	for _, p := range base {
		for _, t := range taken {
			out = append(out, joinSteps(p, t))
		}
		for _, n := range notTaken {
			out = append(out, joinSteps(p, n))
		}
	}
	return out
}

func joinSteps(base, extra []string) []string {
	out := make([]string, 0, len(base)+len(extra))
	out = append(out, base...)
	return append(out, extra...)
}

func addStepToAll(paths [][]string, steps ...string) [][]string {
	for i, p := range paths {
		paths[i] = append(append([]string(nil), p...), steps...)
	}
	return paths
}

// dedupePaths drops duplicate sequences and empty ones (a workflow that
// ran no steps is not a path).
func dedupePaths(paths [][]string) [][]string {
	seen := make(map[string]struct{}, len(paths))
	out := make([][]string, 0, len(paths))
	for _, p := range paths {
		if len(p) == 0 {
			continue
		}
		key := strings.Join(p, "\x00")
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, p)
	}
	return out
}

// stepsIn returns the RunAsStep step names inside the statement, in source
// order. It does not descend into function literals: a step's own body is
// not part of the workflow's path.
func stepsIn(stmt ast.Stmt) []string {
	var steps []string
	ast.Inspect(stmt, func(n ast.Node) bool {
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		call, isCall := n.(*ast.CallExpr)
		if !isCall || calleeName(call.Fun) != "RunAsStep" {
			return true
		}
		steps = append(steps, runAsStepName(call))
		return false
	})
	return steps
}

// calleeName is the call's function name, no matter the import alias:
// dbos.RunAsStep and dbostx.RunAsStep both give "RunAsStep".
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// optionName returns the string literal of the WithXxxName("...") option of
// the call, or false when the option is not present.
func optionName(call *ast.CallExpr, option string) (string, bool) {
	for _, arg := range call.Args {
		opt, isOpt := arg.(*ast.CallExpr)
		if !isOpt || calleeName(opt.Fun) != option || len(opt.Args) != 1 {
			continue
		}
		lit, isLit := opt.Args[0].(*ast.BasicLit)
		if !isLit || lit.Kind != token.STRING {
			continue
		}
		name, _ := strconv.Unquote(lit.Value)
		return name, true
	}
	return "", false
}

// runAsStepName is the step name: the WithStepName literal when present,
// otherwise the step function's name (GreetStep(msg) → "GreetStep").
func runAsStepName(call *ast.CallExpr) string {
	if name, ok := optionName(call, "WithStepName"); ok {
		return name
	}
	if len(call.Args) >= 2 {
		return exprName(call.Args[1])
	}
	return ""
}

func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.CallExpr:
		return exprName(e.Fun)
	}
	return ""
}
