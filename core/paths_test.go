package core_test

import (
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/graph"
)

// SetWorkflowsSource builds the possible-paths graph from the test
// workflows' source (the testworkflows package directory — the twin of
// main.go's //go:embed workflows): every registered workflow is in the
// graph, with its steps and the if forks.
func TestPathsGraphFromTestworkflowsSource(t *testing.T) {
	if err := boot.SetWorkflowsSource(os.DirFS("testworkflows")); err != nil {
		t.Fatalf("SetWorkflowsSource: %v", err)
	}
	g := core.NewPort().Graph()

	// The single-step workflow: one path, one step, a sequence root.
	wg := g.Of("inner-test")
	if wg == nil {
		t.Fatalf("inner-test: nil graph")
	}
	if len(wg.Paths) != 1 || len(wg.Paths[0]) != 1 || wg.Paths[0][0] != "inner" {
		t.Fatalf("inner-test paths: got %v", wg.Paths)
	}
	if len(wg.Steps) != 1 || wg.Steps[0] != "inner" {
		t.Fatalf("inner-test steps: got %v", wg.Steps)
	}
	if wg.Root == nil || wg.Root.Type != graph.BlockSequence {
		t.Fatalf("inner-test root: got %v", wg.Root)
	}

	wg = g.Of("linear-test")
	if wg == nil {
		t.Fatalf("linear-test: nil graph")
	}
	if len(wg.Paths) != 1 || len(wg.Paths[0]) != 4 || wg.Paths[0][0] != "fetch" || wg.Paths[0][3] != "store" {
		t.Fatalf("linear-test paths: got %v", wg.Paths)
	}

	wg = g.Of("fork-test")
	if wg == nil {
		t.Fatalf("fork-test: nil graph")
	}
	if len(wg.Paths) != 2 || len(wg.Paths[0]) != 2 || wg.Paths[0][0] != "base" {
		t.Fatalf("fork-test paths: got %v", wg.Paths)
	}

	// The struct-input workflow: one step (the input is not a path).
	wg = g.Of("struct-test")
	if wg == nil {
		t.Fatalf("struct-test: nil graph")
	}
	if len(wg.Paths) != 1 || len(wg.Paths[0]) != 1 || wg.Paths[0][0] != "schedule" {
		t.Fatalf("struct-test paths: got %v", wg.Paths)
	}

	// The child-dispatch workflow: the RunWorkflow call is a child node (the
	// child func's identifier) after the base step — at run time the child
	// appears in the run's steps as a child row, not as a step row.
	wg = g.Of("child-test")
	if wg == nil {
		t.Fatalf("child-test: nil graph")
	}
	if len(wg.Paths) != 1 || len(wg.Paths[0]) != 2 || wg.Paths[0][0] != "base" || wg.Paths[0][1] != "innerTestWorkflow" {
		t.Fatalf("child-test paths: got %v", wg.Paths)
	}

	// The graph of an unregistered name is nil (no parse, no crash).
	if got := g.Of("ghost"); got != nil {
		t.Fatalf("ghost: got %v, want nil", got)
	}
}
