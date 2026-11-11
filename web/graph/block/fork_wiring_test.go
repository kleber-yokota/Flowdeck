// fork_wiring_test.go pins the fork wiring and the width metrics that the
// fork-width tests do not reach: the incoming fork arrow of a fork that
// follows a box in the main flow (the skip branch forks as arrows only),
// the else-if/else chain of ifBranches, the nested-if width of a branch
// (the fork width plus the padding, the content width plus the padding),
// the empty fork (no first/last boxes), and the level class of the outline
// ramp.
package block

import (
	"strings"
	"testing"

	"flowdeck/core/graph"
)

// A fork that follows a box in the main flow: the incoming fork arrow goes
// from the previous box (a) to each branch's first box — the two arrows of
// the fork (the skip branch is one of them; the branch contents stay
// unconnected inside the fork).
func TestBlockToSVGForkAfterAnArrow(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Node: &graph.Node{Name: "a", Kind: graph.NodeStep}},
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockIf,
				Children: []graph.Child{{Node: &graph.Node{Name: "t", Kind: graph.NodeStep}}},
				Else: &graph.Block{ID: "b2", Level: 1, Type: graph.BlockSequence,
					Children: []graph.Child{{Node: &graph.Node{Name: "f", Kind: graph.NodeStep}}}},
			}},
		}},
	}))
	if !strings.Contains(svg, `>a</text>`) || !strings.Contains(svg, `>t</text>`) || !strings.Contains(svg, `>f</text>`) {
		t.Fatalf("fork after an arrow: want the boxes a t f:\n%s", svg)
	}
	// The only edges are the fork's incoming arrows: from a to the first box
	// of each branch.
	if n := strings.Count(svg, `<line class="g-edge"`); n != 2 {
		t.Fatalf("fork after an arrow: got %d edges, want 2 (a to each branch):\n%s", n, svg)
	}
}

// The else-if chain: the if-body, each else-if body and the else body are
// the branches side by side (three branches here), and the branch that is
// itself a nested if measures as its fork's width (the branches side by
// side) — the nested fork width plus the padding, the content width plus
// the padding.
func TestBlockToSVGForkOfAnElseIfChain(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockIf,
				Children: []graph.Child{
					{Block: &graph.Block{ID: "b6", Level: 2, Type: graph.BlockLoop,
						Children: []graph.Child{{Node: &graph.Node{Name: "x", Kind: graph.NodeStep}}}},
					},
					{Block: &graph.Block{ID: "b2", Level: 2, Type: graph.BlockIf,
						Children: []graph.Child{{Node: &graph.Node{Name: "t", Kind: graph.NodeStep}}},
						Else: &graph.Block{ID: "b3", Level: 2, Type: graph.BlockSequence,
							Children: []graph.Child{{Node: &graph.Node{Name: "f", Kind: graph.NodeStep}}},
						}}}},
				Else: &graph.Block{ID: "b4", Level: 1, Type: graph.BlockIf,
					Children: []graph.Child{{Node: &graph.Node{Name: "u", Kind: graph.NodeStep}}},
					Else: &graph.Block{ID: "b5", Level: 1, Type: graph.BlockSequence,
						Children: []graph.Child{{Node: &graph.Node{Name: "v", Kind: graph.NodeStep}}},
					}},
			}},
		}},
	}))
	for _, want := range []string{">t</text>", ">f</text>", ">u</text>", ">v</text>", ">x</text>"} {
		if !strings.Contains(svg, want) {
			t.Fatalf("else-if fork: missing %s:\n%s", want, svg)
		}
	}
	// The three branch outlines of the nested if (the if and the two
	// branch bodies) plus the outer outlines: the L2 outline of the nested
	// if is drawn.
	if !strings.Contains(svg, `class="g-block l2"`) {
		t.Fatalf("else-if fork: want the L2 outline of the nested if:\n%s", svg)
	}
}

// The empty if (no body, no else): the fork is the skip branch only — no
// boxes at all (the branches' first/last boxes are nil, the fork wires no
// arrows); an if draws no outline of its own, only the root's.
func TestBlockToSVGOfAnEmptyIf(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockIf}},
		}},
	}))
	if strings.Contains(svg, "<text>") {
		t.Fatalf("empty if: want no boxes:\n%s", svg)
	}
	if !strings.Contains(svg, `class="g-block l0"`) {
		t.Fatalf("empty if: want the root outline:\n%s", svg)
	}
}

// levelClass is the outline level ramp: L0, L1, L2, and L3+ reuses L2.
func TestLevelClass(t *testing.T) {
	for level, want := range map[int]string{
		0:  "l0",
		-1: "l0", // defensive: a negative level is the root ramp
		1:  "l1",
		2:  "l2",
		3:  "l2", // L3+ reuses L2
		9:  "l2",
	} {
		if got := levelClass(level); got != want {
			t.Errorf("levelClass(%d): got %q want %q", level, got, want)
		}
	}
}
