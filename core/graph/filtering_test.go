package graph

import (
	"slices"
	"testing"
)

// The construct filters of the block tree: a RunAsStep without a step arg
// (no name), a RunWorkflow without a func arg (no name) and a RunWorkflow
// whose arg is not a name (no name) yield no node; a bare block is
// transparent (its constructs are siblings); a step's own body (a function
// literal) is not walked — only the named constructs of the workflow body
// become nodes.

// A RunAsStep/RunWorkflow without a resolvable name is not a node: the
// named step is the only child, and the read-back is its single path.
func TestBuildGraphFiltersUnnamedConstructs(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx)
	_ = dbos.RunWorkflow(ctx)
	_ = dbos.RunWorkflow(ctx, 42)
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 1 {
		t.Errorf("children: got %d, want 1 (the unnamed calls are filtered): %+v", len(root.Children), root.Children)
		return
	}
	if c := root.Children[0]; c.Node == nil || c.Node.Name != "a" {
		t.Errorf("child 0: got %+v, want the a step", c)
	}
	if !equalPaths(g.Paths, [][]string{{"a"}}) {
		t.Errorf("paths: got %v, want [[a]]", g.Paths)
	}
}

// A bare block is transparent: its constructs become the block's siblings
// (the same nodes, one level shallower in the tree).
func TestBuildGraphBareBlockIsTransparent(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	{
		_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	}
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 1 {
		t.Errorf("children: got %d, want 1: %+v", len(root.Children), root.Children)
		return
	}
	if c := root.Children[0]; c.Node == nil || c.Node.Name != "a" {
		t.Errorf("child 0: got %+v, want the a step", c)
	}
}

// The step's own body is a function literal: its inner RunAsStep is not a
// node of the workflow — only the outer named step is.
func TestBuildGraphSkipsTheStepBody(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	h := func() error { // a function literal: its body is not walked
		_ = dbos.RunAsStep(ctx, innerStep(), dbos.WithStepName("inner"))
		return nil
	}
	_ = h()
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 1 {
		t.Errorf("children: got %d, want 1: %+v", len(root.Children), root.Children)
		return
	}
	if c := root.Children[0]; c.Node == nil || c.Node.Name != "a" {
		t.Errorf("child 0: got %+v, want the a step", c)
	}
	if !slices.Equal(g.Steps, []string{"a"}) {
		t.Errorf("steps: got %v, want [a] (the literal's inner step is not a node)", g.Steps)
	}
}

// hasNode walks the nested blocks and the else: an if whose body has no
// constructs but whose else has one still is a node-bearing block (it is
// kept), and an if nested in an if keeps the outer block node-bearing.
func TestHasNodeNestedAndElse(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	if ok {
		return
	} else {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("b"))
	}
	if ok2 {
		if ok3 {
			_ = dbos.RunAsStep(ctx, cStep(), dbos.WithStepName("c"))
		}
	}
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 2 {
		t.Errorf("children: got %d, want 2 (both ifs keep their nodes): %+v", len(root.Children), root.Children)
		return
	}
	// the first if: its else holds the b step (the hasNode's else walk).
	ifb := root.Children[0].Block
	if ifb == nil || ifb.Type != BlockIf || ifb.Else == nil {
		t.Errorf("child 0: got %+v, want the if with the else", ifb)
		return
	}
	if c := ifb.Else.Children; len(c) != 1 || c[0].Node == nil || c[0].Node.Name != "b" {
		t.Errorf("else child: got %+v, want the b step", c)
	}
	// the second if: the nested if is a node-bearing child block.
	ifb2 := root.Children[1].Block
	if ifb2 == nil || ifb2.Type != BlockIf || len(ifb2.Children) != 1 || ifb2.Children[0].Block == nil {
		t.Errorf("child 1: got %+v, want the if with the nested if", ifb2)
		return
	}
	nested := ifb2.Children[0].Block
	if len(nested.Children) != 1 || nested.Children[0].Node == nil || nested.Children[0].Node.Name != "c" {
		t.Errorf("nested child: got %+v, want the c step", nested.Children)
	}
}
