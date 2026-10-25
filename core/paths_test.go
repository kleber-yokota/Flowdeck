package core_test

import (
	"os"
	"testing"

	"flowdeck/core"
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

	if got := g.Of("hello-world"); len(got) != 1 || len(got[0]) != 1 || got[0][0] != "greet" {
		t.Fatalf("hello-world paths: got %v", got)
	}
	if got := g.Of("pipeline"); len(got) != 1 || len(got[0]) != 4 || got[0][0] != "fetch" || got[0][3] != "store" {
		t.Fatalf("pipeline paths: got %v", got)
	}
	if got := g.Of("decide"); len(got) != 2 || len(got[0]) != 2 || got[0][0] != "base" {
		t.Fatalf("decide paths: got %v", got)
	}
	if got := g.Of("buy"); len(got) != 1 || len(got[0]) != 1 || got[0][0] != "charge" {
		t.Fatalf("buy paths (subpackage): got %v", got)
	}
	// The graph of an unregistered name is empty (no parse, no crash).
	if got := g.Of("ghost"); len(got) != 0 {
		t.Fatalf("ghost paths: got %v", got)
	}
}
