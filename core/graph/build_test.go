package graph

import (
	"slices"
	"testing"
)

// The block tree (the stored possible-paths model): the general build —
// the levels (a loop adds one level), the read-back (the if forks, the
// loop repeats its body once, the steps are the leaves) and the leaf
// node kinds. The special blocks (the if that keeps its level, the
// filtered error guard, the join block) are the build_blocks_test.go
// file's.

// The block tree (the stored possible-paths model): blocks with id · level ·
// type · children, a leaf node with a kind. A loop adds one level; an if
// keeps the level of the block it appears in.
func TestBuildGraphLevels(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("A"))
	for range items {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("B"))
		if ok {
			_ = dbos.RunAsStep(ctx, xStep(), dbos.WithStepName("X"))
		}
		_ = dbos.RunAsStep(ctx, cStep(), dbos.WithStepName("C"))
	}
	for range items {
		_ = dbos.RunAsStep(ctx, dStep(), dbos.WithStepName("D"))
		_ = dbos.RunAsStep(ctx, eStep(), dbos.WithStepName("E"))
	}
	_ = dbos.RunAsStep(ctx, fStep(), dbos.WithStepName("F"))
	return nil
}
`)
	if err != nil {
		t.Errorf("parse snippet: %v", err)
		return
	}
	if body == nil {
		t.Error("snippet: wf func not found")
		return
	}

	g := buildGraph(body)
	root := g.Root
	if root == nil || root.Type != BlockSequence || root.Level != 0 {
		t.Errorf("root: got %+v, want a L0 sequence block", root)
		return
	}
	if len(root.Children) != 4 {
		t.Errorf("root children: got %d, want 4", len(root.Children))
		return
	}
	if c := root.Children[0]; c.Node == nil || c.Node.Name != "A" || c.Node.Kind != NodeStep {
		t.Errorf("child 0: got %+v, want the A step", c)
	}
	b1 := root.Children[1]
	if b1.Block == nil || b1.Block.Type != BlockLoop || b1.Block.Level != 1 {
		t.Errorf("child 1: got %+v, want an L1 loop block", b1)
		return
	}
	if len(b1.Block.Children) != 3 {
		t.Errorf("b1 children: got %d, want 3", len(b1.Block.Children))
		return
	}
	b3 := b1.Block.Children[1]
	if b3.Block == nil || b3.Block.Type != BlockIf || b3.Block.Level != 1 {
		t.Errorf("b3: got %+v, want an L1 if block (it keeps the loop's level)", b3)
		return
	}
	if x := b3.Block.Children[0]; x.Node == nil || x.Node.Name != "X" {
		t.Errorf("b3 child: got %+v, want the X step", x)
	}
	b2 := root.Children[2]
	if b2.Block == nil || b2.Block.Type != BlockLoop || b2.Block.Level != 1 {
		t.Errorf("child 2: got %+v, want an L1 loop block", b2)
	}
	if f := root.Children[3]; f.Node == nil || f.Node.Name != "F" {
		t.Errorf("child 3: got %+v, want the F step", f)
	}
}

// The read-back: the if forks (with the if step and without it), the loop
// repeats its body once (no duplication), and the steps are the leaves.
func TestBuildGraphReadBack(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("A"))
	for range items {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("B"))
		if ok {
			_ = dbos.RunAsStep(ctx, xStep(), dbos.WithStepName("X"))
		}
		_ = dbos.RunAsStep(ctx, cStep(), dbos.WithStepName("C"))
	}
	for range items {
		_ = dbos.RunAsStep(ctx, dStep(), dbos.WithStepName("D"))
		_ = dbos.RunAsStep(ctx, eStep(), dbos.WithStepName("E"))
	}
	_ = dbos.RunAsStep(ctx, fStep(), dbos.WithStepName("F"))
	return nil
}
`)
	if err != nil {
		t.Errorf("parse snippet: %v", err)
		return
	}
	if body == nil {
		t.Error("snippet: wf func not found")
		return
	}

	g := buildGraph(body)
	if !equalPaths(g.Paths, [][]string{
		{"A", "B", "X", "C", "D", "E", "F"},
		{"A", "B", "C", "D", "E", "F"},
	}) {
		t.Errorf("paths: got %v", g.Paths)
	}
	if !slices.Equal(g.Steps, []string{"A", "B", "X", "C", "D", "E", "F"}) {
		t.Errorf("steps: got %v", g.Steps)
	}
}

// The leaf node kinds: RunAsStep (step) · Go (async) · RunWorkflow (child) ·
// Sleep · Recv (wait).
func TestBuildGraphNodeKinds(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("s"))
	_ = dbos.Go(ctx, asyncStep(), dbos.WithStepName("a"))
	_ = dbos.RunWorkflow(ctx, child, "in")
	_, _ = dbos.Sleep(ctx, time.Second)
	_, _ = dbos.Recv(ctx, "topic", time.Second)
	return nil
}
`)
	if err != nil {
		t.Errorf("parse snippet: %v", err)
		return
	}
	if body == nil {
		t.Error("snippet: wf func not found")
		return
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 5 {
		t.Errorf("children: got %d, want 5", len(root.Children))
		return
	}
	var kinds []NodeKind
	var names []string
	for _, c := range root.Children {
		if c.Node == nil {
			t.Errorf("child is a block, want a node: %+v", c)
			return
		}
		kinds = append(kinds, c.Node.Kind)
		names = append(names, c.Node.Name)
	}
	if !slices.Equal(kinds, []NodeKind{NodeStep, NodeAsync, NodeChild, NodeWait, NodeWait}) {
		t.Errorf("kinds: got %v", kinds)
	}
	if !slices.Equal(names, []string{"s", "a", "child", "sleep", "recv"}) {
		t.Errorf("names: got %v", names)
	}
}
