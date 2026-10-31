package graph

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// pathsFromSources parses the given Go source files and builds the maps:
// workflow name → all possible paths, and workflow name → parameter spec
// (input + step defaults). The marker is the per-file DBOS registration
// RegisterWorkflow(ctx, Func, WithWorkflowName("name"), ...) — matched by
// function name, so any import alias of the dbos package is found. A
// RunAsStep call adds a step to every open path; an if forks the paths
// (taken branch vs not-taken branch).
func pathsFromSources(files map[string][]byte) (map[string]*WorkflowGraph, map[string]Spec, error) {
	fset := token.NewFileSet()
	funcs := map[string]*ast.FuncDecl{}
	regs := map[string]string{} // registered name → workflow func name
	funcPkg := map[string]string{}
	pkgConstants := map[string]map[string]string{}
	pkgTypes := map[string]map[string][]Param{}
	for name, data := range files {
		file, err := parser.ParseFile(fset, name, data, 0)
		if err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", name, err)
		}
		collectFuncs(file, funcs, funcPkg)
		collectRegistrations(file, regs)
		collectConstants(file, pkgConstants)
		collectTypes(file, pkgTypes)
	}
	return pathsFromRegistrations(regs, funcs),
		specsFromRegistrations(regs, funcs, funcPkg, pkgConstants, pkgTypes), nil
}

// collectFuncs records the file's function declarations (the workflow
// func bodies are walked later, by possiblePaths) and the package each
// func is declared in.
func collectFuncs(file *ast.File, funcs map[string]*ast.FuncDecl, funcPkg map[string]string) {
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok {
			funcs[fd.Name.Name] = fd
			funcPkg[fd.Name.Name] = file.Name.Name
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

// possiblePaths returns every sequence of steps that can run in the
// function: the stored block tree read back (each RunAsStep adds a step to
// every open path, each if forks). Empty sequences and duplicates are
// dropped.
func possiblePaths(body *ast.BlockStmt) [][]string {
	return buildGraph(body).Paths
}

// buildGraph builds the block tree of the workflow body (the stored
// possible-paths model): the root is a sequence block (L0), a block has a
// type (sequence · if · loop) and a level (a loop adds one level, an if
// keeps the level of the block it appears in), and a leaf node has a kind
// (step · async · child · wait). The read-back paths (Paths) and the steps
// (Steps) are derived from the stored tree.
func buildGraph(body *ast.BlockStmt) *WorkflowGraph {
	g := &idGen{}
	root := &Block{ID: g.next(), Level: 0, Type: BlockSequence}
	root.Children = buildChildren(body.List, 0, g)
	return &WorkflowGraph{
		Root:  root,
		Steps: stepsOf(root),
		Paths: readPaths(root),
	}
}

// idGen assigns the block ids (b0, b1, ...) in build order.
type idGen struct{ n int }

func (g *idGen) next() string {
	id := "b" + strconv.Itoa(g.n)
	g.n++
	return id
}

// buildChildren builds the ordered children of the block at the given level
// (the level of the block whose children these are): a for / for range is a
// loop block (one level deeper), an if is an if block (it keeps the level),
// a bare {} block is transparent, and any other statement yields its
// constructs (the leaf nodes) in source order.
func buildChildren(stmts []ast.Stmt, level int, g *idGen) []Child {
	var children []Child
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.BlockStmt:
			children = append(children, buildChildren(s.List, level, g)...)
		case *ast.IfStmt:
			children = append(children, blockChild(buildIf(s, level, g))...)
		case *ast.ForStmt:
			children = append(children, blockChild(buildLoop(s.Body, level, g))...)
		case *ast.RangeStmt:
			children = append(children, blockChild(buildLoop(s.Body, level, g))...)
		default:
			for _, n := range constructsIn(stmt) {
				children = append(children, Child{Node: n})
			}
		}
	}
	return groupJoins(children, level, g)
}

// groupJoins wraps each run of async nodes right before a channel wait (a
// select or a recv) in a join block: the asyncs and the wait that joins
// them, one level deeper.
func groupJoins(children []Child, level int, g *idGen) []Child {
	var out []Child
	for i := 0; i < len(children); {
		j := i
		for j < len(children) && isAsyncChild(children[j]) {
			j++
		}
		if j > i && isJoinWait(children[j]) {
			join := &Block{ID: g.next(), Level: level + 1, Type: BlockJoin}
			join.Children = append(join.Children, children[i:j]...)
			join.Children = append(join.Children, children[j])
			out = append(out, Child{Block: join})
			i = j + 1
			continue
		}
		out = append(out, children[i])
		i++
	}
	return out
}

// isAsyncChild reports whether the child is an async node.
func isAsyncChild(c Child) bool {
	return c.Node != nil && c.Node.Kind == NodeAsync
}

// isJoinWait reports whether the child is the channel wait that joins an
// async run (a select or a recv — a sleep is a timer, not a join).
func isJoinWait(c Child) bool {
	if c.Node == nil || c.Node.Kind != NodeWait {
		return false
	}
	return c.Node.Name == "select" || c.Node.Name == "recv"
}

// blockChild returns the block child if it has content — an error guard (an
// if or a loop with no workflow constructs, like `if err != nil { return }`)
// has none and is filtered out of the tree.
func blockChild(b *Block) []Child {
	if hasNode(b) {
		return []Child{{Block: b}}
	}
	return nil
}

// hasNode reports whether the block (or any descendant) contains a node.
func hasNode(b *Block) bool {
	for _, c := range b.Children {
		if c.Node != nil {
			return true
		}
		if c.Block != nil && hasNode(c.Block) {
			return true
		}
	}
	if b.Else != nil && hasNode(b.Else) {
		return true
	}
	return false
}

// buildIf builds the if block: the if-body is Children (built at the if's
// level, which keeps the level of the block it appears in); the else is Else
// (an else-if is another if block, an else body is a sequence block, no else
// is nil).
func buildIf(s *ast.IfStmt, level int, g *idGen) *Block {
	b := &Block{ID: g.next(), Level: level, Type: BlockIf}
	b.Children = buildChildren(s.Body.List, level, g)
	switch e := s.Else.(type) {
	case *ast.IfStmt:
		if eb := buildIf(e, level, g); hasNode(eb) {
			b.Else = eb
		}
	case *ast.BlockStmt:
		eb := &Block{ID: g.next(), Level: level, Type: BlockSequence}
		eb.Children = buildChildren(e.List, level, g)
		if hasNode(eb) {
			b.Else = eb
		}
	}
	return b
}

// buildLoop builds the loop block: a for / for range, one level deeper than
// the block it appears in.
func buildLoop(body *ast.BlockStmt, level int, g *idGen) *Block {
	b := &Block{ID: g.next(), Level: level + 1, Type: BlockLoop}
	b.Children = buildChildren(body.List, level+1, g)
	return b
}

// constructsIn returns the DBOS constructs in the statement, in source
// order, as leaf nodes. It does not descend into function literals: a
// step's own body is not part of the workflow's path.
func constructsIn(stmt ast.Stmt) []*Node {
	var nodes []*Node
	ast.Inspect(stmt, func(n ast.Node) bool {
		if _, isLit := n.(*ast.FuncLit); isLit {
			return false
		}
		call, isCall := n.(*ast.CallExpr)
		if !isCall {
			return true
		}
		if name, kind, ok := nodeName(call); ok {
			nodes = append(nodes, &Node{Name: name, Kind: kind})
		}
		return false
	})
	return nodes
}

// waitNames is the fixed name of each wait construct (the callee name → the
// node name).
var waitNames = map[string]string{
	"Sleep":  "sleep",
	"Recv":   "recv",
	"Select": "select",
}

// nodeName is the leaf node of one DBOS construct call: the name (the
// WithStepName literal when present, else the called function's name; the
// fixed name for Sleep · Recv · Select) and the kind.
func nodeName(call *ast.CallExpr) (string, NodeKind, bool) {
	switch calleeName(call.Fun) {
	case "RunAsStep":
		return namedNode(call, NodeStep)
	case "Go":
		return namedNode(call, NodeAsync)
	case "RunWorkflow":
		return childNode(call)
	}
	if name, ok := waitNames[calleeName(call.Fun)]; ok {
		return name, NodeWait, true
	}
	return "", "", false
}

// namedNode is the name of a RunAsStep/Go call (the WithStepName literal
// when present, else the called function's name) and its kind.
func namedNode(call *ast.CallExpr, kind NodeKind) (string, NodeKind, bool) {
	name := runAsStepName(call)
	if name == "" {
		return "", "", false
	}
	return name, kind, true
}

// childNode is the name of a RunWorkflow call (the child workflow func's
// name) and its kind.
func childNode(call *ast.CallExpr) (string, NodeKind, bool) {
	if len(call.Args) < 2 {
		return "", "", false
	}
	name := exprName(call.Args[1])
	if name == "" {
		return "", "", false
	}
	return name, NodeChild, true
}

// readPaths returns every sequence of step names through the block: the
// stored tree read back (a node appends its name to every open path, a
// sequence/loop block's children extend every open path, an if forks). Empty
// sequences and duplicates are dropped.
func readPaths(b *Block) [][]string {
	return dedupePaths(readBlock(b, [][]string{{}}))
}

// readBlock reads the block's children into the open paths (forking at the
// nested ifs).
func readBlock(b *Block, open [][]string) [][]string {
	for _, c := range b.Children {
		open = readChild(c, open)
	}
	return open
}

// readChild reads one child into the open paths: a node appends its name, an
// if forks (taken vs not-taken), a sequence/loop block's children extend
// every open path.
func readChild(c Child, open [][]string) [][]string {
	if c.Node != nil {
		return addStepToAll(open, c.Node.Name)
	}
	b := c.Block
	if b.Type == BlockIf {
		taken := readBlock(b, [][]string{{}})
		notTaken := blockSequences(b.Else)
		return forkPaths(open, taken, notTaken)
	}
	return readBlock(b, open)
}

// blockSequences returns the sequences of step names through the else (the
// not-taken branch of an if): nil is the empty sequence, an else-if forks, an
// else body extends.
func blockSequences(b *Block) [][]string {
	if b == nil {
		return [][]string{{}}
	}
	if b.Type == BlockIf {
		return forkPaths([][]string{{}}, readBlock(b, [][]string{{}}), blockSequences(b.Else))
	}
	return readBlock(b, [][]string{{}})
}

// stepsOf returns the leaf node names of the block tree, in build order.
func stepsOf(b *Block) []string {
	var steps []string
	var walk func(bl *Block)
	walk = func(bl *Block) {
		for _, c := range bl.Children {
			if c.Node != nil {
				steps = append(steps, c.Node.Name)
			} else {
				walk(c.Block)
			}
		}
		if bl.Else != nil {
			walk(bl.Else)
		}
	}
	walk(b)
	return steps
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
