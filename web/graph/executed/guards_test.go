// guards_test.go pins the empty-path and defensive layout branches of the
// executed graph: the empty path (no boxes at all), the loop group whose
// last member is a child row (the group's badge state is the child's live
// state), the tail of a path with no box (the run just started), and the
// group layout skipping a terminal member (the defensive skip of the
// invisible end marker inside a group).
package executed

import (
	"testing"
	"time"

	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// groupPath of no steps is nil (no boxes, no root) — the empty run's path.
func TestGroupPathOfNoStepsIsNil(t *testing.T) {
	if roots := groupPath(nil); roots != nil {
		t.Fatalf("groupPath(nil): got %v, want nil", roots)
	}
	if roots := groupPath([]reads.Step{}); roots != nil {
		t.Fatalf("groupPath([]): got %v, want nil", roots)
	}
}

// The loop group of repeated child dispatches: the consecutive rows of the
// same name with a child row last take the child's name and the child's
// live state as the group's badge (the dispatch's name and state only
// when the child's name is missing — the other file covers that fallback).
func TestGroupBoxOfAChildMember(t *testing.T) {
	start := time.Now()
	rows := []reads.Step{
		{Name: "dispatch", Kind: reads.KindStep, State: status.Success, StartedAt: start, CompletedAt: start.Add(time.Second)},
		{Name: "dispatch", Kind: reads.KindChild, ChildName: "num-test", ChildState: status.Success, StartedAt: start, CompletedAt: start.Add(time.Second)},
	}
	roots := groupPath(rows)
	if len(roots) != 1 {
		t.Fatalf("groupPath: got %d roots, want 1", len(roots))
	}
	n := roots[0]
	if n.Step != "num-test ×2" {
		t.Fatalf("group box: got %q, want %q", n.Step, "num-test ×2")
	}
	if n.State != string(status.Success) {
		t.Fatalf("group state: got %q, want the child's state", n.State)
	}
}

// withTail of an empty path is just the dashed tail box (the run just
// started — no step has completed yet).
func TestWithTailOfAnEmptyPath(t *testing.T) {
	roots := withTail(nil)
	if len(roots) != 1 || !roots[0].Tail || roots[0].Step != "…" {
		t.Fatalf("withTail(nil): got %v, want the tail box only", roots)
	}
}

// placeGroup skips the terminal marker among the group's members (the
// defensive skip: the invisible end marker never gets a box of its own).
func TestPlaceGroupSkipsATerminalMember(t *testing.T) {
	roots := []pathNode{{Children: []pathNode{{Terminal: true}, {Step: "a"}}}}
	g := newGraphLayout(roots)
	if len(g.nodes) != 2 {
		// The group node itself plus its one visible member.
		t.Fatalf("nodes: got %d, want 2 (the group and the member)", len(g.nodes))
	}
}
