package graph

import (
	"slices"
	"testing"
)

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
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	g := buildGraph(body)
	root := g.Root
	if root == nil || root.Type != BlockSequence || root.Level != 0 {
		t.Fatalf("root: got %+v, want a L0 sequence block", root)
	}
	if len(root.Children) != 4 {
		t.Fatalf("root children: got %d, want 4", len(root.Children))
	}
	if c := root.Children[0]; c.Node == nil || c.Node.Name != "A" || c.Node.Kind != NodeStep {
		t.Fatalf("child 0: got %+v, want the A step", c)
	}
	b1 := root.Children[1]
	if b1.Block == nil || b1.Block.Type != BlockLoop || b1.Block.Level != 1 {
		t.Fatalf("child 1: got %+v, want an L1 loop block", b1)
	}
	if len(b1.Block.Children) != 3 {
		t.Fatalf("b1 children: got %d, want 3", len(b1.Block.Children))
	}
	b3 := b1.Block.Children[1]
	if b3.Block == nil || b3.Block.Type != BlockIf || b3.Block.Level != 1 {
		t.Fatalf("b3: got %+v, want an L1 if block (it keeps the loop's level)", b3)
	}
	if x := b3.Block.Children[0]; x.Node == nil || x.Node.Name != "X" {
		t.Fatalf("b3 child: got %+v, want the X step", x)
	}
	b2 := root.Children[2]
	if b2.Block == nil || b2.Block.Type != BlockLoop || b2.Block.Level != 1 {
		t.Fatalf("child 2: got %+v, want an L1 loop block", b2)
	}
	if f := root.Children[3]; f.Node == nil || f.Node.Name != "F" {
		t.Fatalf("child 3: got %+v, want the F step", f)
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
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	g := buildGraph(body)
	if !equalPaths(g.Paths, [][]string{
		{"A", "B", "X", "C", "D", "E", "F"},
		{"A", "B", "C", "D", "E", "F"},
	}) {
		t.Fatalf("paths: got %v", g.Paths)
	}
	if !slices.Equal(g.Steps, []string{"A", "B", "X", "C", "D", "E", "F"}) {
		t.Fatalf("steps: got %v", g.Steps)
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
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 5 {
		t.Fatalf("children: got %d, want 5", len(root.Children))
	}
	var kinds []NodeKind
	var names []string
	for _, c := range root.Children {
		if c.Node == nil {
			t.Fatalf("child is a block, want a node: %+v", c)
		}
		kinds = append(kinds, c.Node.Kind)
		names = append(names, c.Node.Name)
	}
	if !slices.Equal(kinds, []NodeKind{NodeStep, NodeAsync, NodeChild, NodeWait, NodeWait}) {
		t.Fatalf("kinds: got %v", kinds)
	}
	if !slices.Equal(names, []string{"s", "a", "child", "sleep", "recv"}) {
		t.Fatalf("names: got %v", names)
	}
}

// An if keeps the level of the block it appears in (an if at the root is L0),
// and the else body is a sequence block at the if's level.
func TestBuildGraphIfKeepsLevel(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	if ok {
		_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	} else {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("b"))
	}
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	g := buildGraph(body)
	root := g.Root
	ifb := root.Children[0]
	if ifb.Block == nil || ifb.Block.Type != BlockIf || ifb.Block.Level != 0 {
		t.Fatalf("if at root: got %+v, want an L0 if block", ifb)
	}
	elseb := ifb.Block.Else
	if elseb == nil || elseb.Type != BlockSequence || elseb.Level != 0 {
		t.Fatalf("else body: got %+v, want an L0 sequence block", elseb)
	}
	if b := elseb.Children[0]; b.Node == nil || b.Node.Name != "b" {
		t.Fatalf("else child: got %+v, want the b step", b)
	}
}

// An error guard (an if with a return and no else) has no workflow
// constructs and is not a fork — it is filtered out of the tree, so the
// steps are the root's direct children and the read-back is a single path.
func TestBuildGraphErrorGuardNoBlock(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_, err := dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("fetch"))
	if err != nil {
		return err
	}
	_, err = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("store"))
	if err != nil {
		return err
	}
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 2 {
		t.Fatalf("root children: got %d, want 2 (the guards are filtered)", len(root.Children))
	}
	if c := root.Children[0]; c.Node == nil || c.Node.Name != "fetch" {
		t.Fatalf("child 0: got %+v, want the fetch step", c)
	}
	if c := root.Children[1]; c.Node == nil || c.Node.Name != "store" {
		t.Fatalf("child 1: got %+v, want the store step", c)
	}
	if !equalPaths(g.Paths, [][]string{{"fetch", "store"}}) {
		t.Fatalf("paths: got %v, want [[fetch store]]", g.Paths)
	}
}

// An async run right before a channel wait (a select / a recv) is a join
// block: the asyncs and the wait that joins them, one level deeper.
func TestBuildGraphJoinBlock(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.Go(ctx, aStep(), dbos.WithStepName("alpha"))
	_ = dbos.Go(ctx, aStep(), dbos.WithStepName("beta"))
	_, _ = dbos.Recv(ctx, "topic", time.Second)
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 1 {
		t.Fatalf("root children: got %d, want 1 (the join block)", len(root.Children))
	}
	join := root.Children[0].Block
	if join == nil || join.Type != BlockJoin || join.Level != 1 {
		t.Fatalf("child 0: got %+v, want the L1 join block", root.Children[0].Block)
	}
	var names []string
	for _, c := range join.Children {
		if c.Node == nil {
			t.Fatalf("join child is a block, want a node: %+v", c)
		}
		names = append(names, c.Node.Name)
	}
	if !slices.Equal(names, []string{"alpha", "beta", "recv"}) {
		t.Fatalf("join children: got %v", names)
	}
	if !equalPaths(g.Paths, [][]string{{"alpha", "beta", "recv"}}) {
		t.Fatalf("paths: got %v", g.Paths)
	}
}

// No join: an async run with nothing right after it, and a sleep right
// after an async (a sleep is a timer, not a join) stay flat.
func TestBuildGraphNoJoin(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.Go(ctx, aStep(), dbos.WithStepName("a"))
	_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("s"))
	_ = dbos.Go(ctx, aStep(), dbos.WithStepName("b"))
	_, _ = dbos.Sleep(ctx, time.Second)
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 4 {
		t.Fatalf("root children: got %d, want 4 (no join blocks)", len(root.Children))
	}
	for i, c := range root.Children {
		if c.Node == nil {
			t.Fatalf("child %d is a block, want a node: %+v", i, c)
		}
	}
}
