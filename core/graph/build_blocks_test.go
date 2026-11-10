package graph

import (
	"slices"
	"testing"
)

// The special blocks of the block tree (the stored possible-paths model):
// the if that keeps the level of the block it appears in (and its else
// body), the filtered error guard, and the join block (the asyncs and the
// wait that joins them).

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
		t.Errorf("parse snippet: %v", err)
		return
	}
	if body == nil {
		t.Error("snippet: wf func not found")
		return
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 1 {
		t.Errorf("root children: got %d, want 1", len(root.Children))
		return
	}
	ifb := root.Children[0]
	if ifb.Block == nil || ifb.Block.Type != BlockIf || ifb.Block.Level != 0 {
		t.Errorf("if at root: got %+v, want an L0 if block", ifb)
		return
	}
	elseb := ifb.Block.Else
	if elseb == nil || elseb.Type != BlockSequence || elseb.Level != 0 {
		t.Errorf("else body: got %+v, want an L0 sequence block", elseb)
		return
	}
	if b := elseb.Children[0]; b.Node == nil || b.Node.Name != "b" {
		t.Errorf("else child: got %+v, want the b step", b)
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
		t.Errorf("parse snippet: %v", err)
		return
	}
	if body == nil {
		t.Error("snippet: wf func not found")
		return
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 2 {
		t.Errorf("root children: got %d, want 2 (the guards are filtered)", len(root.Children))
		return
	}
	if c := root.Children[0]; c.Node == nil || c.Node.Name != "fetch" {
		t.Errorf("child 0: got %+v, want the fetch step", c)
	}
	if c := root.Children[1]; c.Node == nil || c.Node.Name != "store" {
		t.Errorf("child 1: got %+v, want the store step", c)
	}
	if !equalPaths(g.Paths, [][]string{{"fetch", "store"}}) {
		t.Errorf("paths: got %v, want [[fetch store]]", g.Paths)
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
		t.Errorf("parse snippet: %v", err)
		return
	}
	if body == nil {
		t.Error("snippet: wf func not found")
		return
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 1 {
		t.Errorf("root children: got %d, want 1 (the join block)", len(root.Children))
		return
	}
	join := root.Children[0].Block
	if join == nil || join.Type != BlockJoin || join.Level != 1 {
		t.Errorf("child 0: got %+v, want the L1 join block", join)
		return
	}
	var names []string
	for _, c := range join.Children {
		if c.Node == nil {
			t.Errorf("join child is a block, want a node: %+v", c)
			return
		}
		names = append(names, c.Node.Name)
	}
	if !slices.Equal(names, []string{"alpha", "beta", "recv"}) {
		t.Errorf("join children: got %v", names)
	}
	if !equalPaths(g.Paths, [][]string{{"alpha", "beta", "recv"}}) {
		t.Errorf("paths: got %v", g.Paths)
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
		t.Errorf("parse snippet: %v", err)
		return
	}
	if body == nil {
		t.Error("snippet: wf func not found")
		return
	}

	g := buildGraph(body)
	root := g.Root
	if len(root.Children) != 4 {
		t.Errorf("root children: got %d, want 4 (no join blocks)", len(root.Children))
		return
	}
	for i, c := range root.Children {
		if c.Node == nil {
			t.Errorf("child %d is a block, want a node: %+v", i, c)
		}
	}
}
