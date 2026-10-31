package core_test

import (
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/graph"
)

// SetWorkflowsSource builds the possible-paths graph from the workflows
// tree (the unit twin of main.go's //go:embed workflows): every registered
// workflow is in the graph, with its steps and the if forks — including
// the subpackage workflow, found by the recursive walk of the tree.
func TestPathsGraphFromWorkflowsTree(t *testing.T) {
	if err := core.SetWorkflowsSource(os.DirFS("../workflows")); err != nil {
		t.Fatalf("SetWorkflowsSource: %v", err)
	}
	g := core.NewPort().Graph

	wg := g.Of("hello-world")
	if wg == nil {
		t.Fatalf("hello-world: nil graph")
	}
	if len(wg.Paths) != 1 || len(wg.Paths[0]) != 1 || wg.Paths[0][0] != "greet" {
		t.Fatalf("hello-world paths: got %v", wg.Paths)
	}
	if len(wg.Steps) != 1 || wg.Steps[0] != "greet" {
		t.Fatalf("hello-world steps: got %v", wg.Steps)
	}
	if wg.Root == nil || wg.Root.Type != graph.BlockSequence {
		t.Fatalf("hello-world root: got %v", wg.Root)
	}

	wg = g.Of("pipeline")
	if wg == nil {
		t.Fatalf("pipeline: nil graph")
	}
	if len(wg.Paths) != 1 || len(wg.Paths[0]) != 4 || wg.Paths[0][0] != "fetch" || wg.Paths[0][3] != "store" {
		t.Fatalf("pipeline paths: got %v", wg.Paths)
	}

	wg = g.Of("decide")
	if wg == nil {
		t.Fatalf("decide: nil graph")
	}
	if len(wg.Paths) != 2 || len(wg.Paths[0]) != 2 || wg.Paths[0][0] != "base" {
		t.Fatalf("decide paths: got %v", wg.Paths)
	}

	wg = g.Of("buy")
	if wg == nil {
		t.Fatalf("buy: nil graph")
	}
	if len(wg.Paths) != 1 || len(wg.Paths[0]) != 1 || wg.Paths[0][0] != "charge" {
		t.Fatalf("buy paths: got %v", wg.Paths)
	}

	// The graph of an unregistered name is nil (no parse, no crash).
	if got := g.Of("ghost"); got != nil {
		t.Fatalf("ghost: got %v, want nil", got)
	}
}
