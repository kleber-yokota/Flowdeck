package graph

import (
	"os"
	"slices"
	"testing"
)

// The boot graph build from the FS (New): the production-tree parse gate.

// TestGraphBuild is the graph-build gate (package-map #gates): it builds the
// possible-paths graph from the production workflows tree (the files on
// disk, no boot) and asserts the parse covers the spec, the steps, the
// child, the loop and the wait — a workflow change that breaks the AST
// parse fails the build.
func TestGraphBuild(t *testing.T) {
	g, err := New(os.DirFS("../../workflows"))
	if err != nil {
		t.Errorf("build graph from the production tree: %v", err)
		return
	}

	// every built workflow has at least one path and at least one step node
	// (a leaf node of the tree — any kind: the fanout tree has only the
	// asyncs and the select wait, no RunAsStep).
	if len(g.ByName) == 0 {
		t.Error("ByName: no workflows built from the production tree")
		return
	}
	for name, wg := range g.ByName {
		if len(wg.Paths) == 0 {
			t.Errorf("workflow %q: no paths", name)
		}
		if len(wg.Steps) == 0 {
			t.Errorf("workflow %q: no step node in the tree", name)
		}
	}

	// the required graphs (the parse must have built them) — the tree
	// assertions below read them directly.
	missing := false
	for _, name := range []string{"pipeline", "compose", "loop", "fanout"} {
		if g.ByName[name] == nil {
			t.Errorf("ByName: missing the %q graph", name)
			missing = true
		}
	}
	if missing {
		return
	}

	// the pipeline: the spec (the msg input with its default) and the
	// path (fetch · parse · transform · store).
	spec := g.Specs["pipeline"]
	if len(spec.Inputs) != 1 || spec.Inputs[0].Name != "msg" || spec.Inputs[0].Default != "FlowDeck" {
		t.Errorf("pipeline spec: got %+v, want the msg input with the FlowDeck default", spec)
	}
	if !treeHasPath(g.ByName["pipeline"], []string{"fetch", "parse", "transform", "store"}) {
		t.Errorf("pipeline paths: got %v, want the fetch · parse · transform · store path", g.ByName["pipeline"].Paths)
	}

	// the child (the Pipeline node in the compose tree), the loop (a loop
	// block in the loop tree) and the wait (the select node in the fanout
	// tree).
	if !treeHasNode(g.ByName["compose"].Root, "Pipeline", NodeChild) {
		t.Error("compose tree: no child node named Pipeline")
	}
	if !treeHasBlock(g.ByName["loop"].Root, BlockLoop) {
		t.Error("loop tree: no loop block")
	}
	if !treeHasNode(g.ByName["fanout"].Root, "select", NodeWait) {
		t.Error("fanout tree: no wait node named select")
	}
}

// walkTree collects the block tree's leaf nodes (nodes) and its blocks
// (blocks).
func walkTree(root *Block, nodes *[]*Node, blocks *[]*Block) {
	var walk func(b *Block)
	walk = func(b *Block) {
		if blocks != nil {
			*blocks = append(*blocks, b)
		}
		for _, c := range b.Children {
			if c.Node != nil {
				if nodes != nil {
					*nodes = append(*nodes, c.Node)
				}
			} else if c.Block != nil {
				walk(c.Block)
			}
		}
		if b.Else != nil {
			walk(b.Else)
		}
	}
	walk(root)
}

// treeHasNode reports whether the tree has a node with the name and the
// kind.
func treeHasNode(root *Block, name string, kind NodeKind) bool {
	var nodes []*Node
	walkTree(root, &nodes, nil)
	for _, n := range nodes {
		if n.Name == name && n.Kind == kind {
			return true
		}
	}
	return false
}

// treeHasBlock reports whether the tree has a block of the type.
func treeHasBlock(root *Block, bt BlockType) bool {
	var blocks []*Block
	walkTree(root, nil, &blocks)
	for _, b := range blocks {
		if b.Type == bt {
			return true
		}
	}
	return false
}

// treeHasPath reports whether the read-back paths contain the exact path.
func treeHasPath(wg *WorkflowGraph, want []string) bool {
	for _, p := range wg.Paths {
		if slices.Equal(p, want) {
			return true
		}
	}
	return false
}
