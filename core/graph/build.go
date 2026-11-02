package graph

import (
	"go/ast"
	"strconv"
)

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
