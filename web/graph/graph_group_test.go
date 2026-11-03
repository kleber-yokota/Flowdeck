// graph_group_test.go asserts the executed-path grouping (task 02): the
// plain box · the parallel group (overlap → the ∥ mark) · the loop group
// (repeated name → the arc + ×N) · the collapse (> 4 members → one
// annotated box, the full list in <title>) · the child box · the wait box.
package graph

import (
	"strings"
	"testing"
	"time"

	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// TestGroupPathPlainStepsAreOneBoxEach: a run without parallelism is the
// top-down chain of 48px boxes, one per step, labeled with the step name.
func TestGroupPathPlainStepsAreOneBoxEach(t *testing.T) {
	steps := []reads.Step{
		{Name: "fetch", State: status.Success},
		{Name: "parse", State: status.Error},
	}
	g := groupPath(steps)
	if len(g) != 1 || len(g[0].Children) != 2 {
		t.Fatalf("plain: want the chain of two nodes, got %v", g)
	}
	if g[0].Step != "fetch" || g[0].Children[0].Step != "parse" {
		t.Fatalf("plain: want fetch → parse, got %q → %q", g[0].Step, g[0].Children[0].Step)
	}
	svg := string(ExecutedPathToSVG(steps))
	if n := strings.Count(svg, `<rect class="g-node`); n != 2 {
		t.Fatalf("plain: got %d boxes, want 2:\n%s", n, svg)
	}
	if !strings.Contains(svg, `height="48.0"`) {
		t.Fatalf("plain: want the 48px boxes:\n%s", svg)
	}
	if !strings.Contains(svg, `>fetch</text>`) || !strings.Contains(svg, `>parse</text>`) {
		t.Fatalf("plain: want the name labels:\n%s", svg)
	}
}

// TestGroupPathOverlapIsAParallelGroup: the rows whose [StartedAt,
// CompletedAt] intervals overlap (runtime parallelism — the fanout
// go+select members) are the parallel group: both member boxes, and the ∥
// mark (async-symbol) between them.
func TestGroupPathOverlapIsAParallelGroup(t *testing.T) {
	base := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	steps := []reads.Step{
		{Name: "fetch", State: status.Success, StartedAt: base, CompletedAt: base.Add(100 * time.Millisecond)},
		{Name: "alpha", State: status.Success, StartedAt: base.Add(200 * time.Millisecond), CompletedAt: base.Add(500 * time.Millisecond)},
		{Name: "beta", State: status.Success, StartedAt: base.Add(200 * time.Millisecond), CompletedAt: base.Add(400 * time.Millisecond)},
	}
	g := groupPath(steps)
	// fetch → (the invisible group node) → {alpha, beta}
	if len(g) != 1 || g[0].Step != "fetch" || len(g[0].Children) != 1 {
		t.Fatalf("parallel: want fetch above the group, got %v", g)
	}
	grp := g[0].Children[0]
	if grp.Step != "" || len(grp.Children) != 2 {
		t.Fatalf("parallel: want the group node with two members, got %+v", grp)
	}
	svg := string(ExecutedPathToSVG(steps))
	if !strings.Contains(svg, `>alpha</text>`) || !strings.Contains(svg, `>beta</text>`) {
		t.Fatalf("parallel: want both member boxes:\n%s", svg)
	}
	if !strings.Contains(svg, `class="async-symbol"`) {
		t.Fatalf("parallel: want the ∥ mark (async-symbol):\n%s", svg)
	}
}

// TestGroupPathRepeatedNameIsALoop: consecutive rows of the repeated step
// name (the loop body) are the loop group — one box, the loop-symbol arc,
// the ×N count.
func TestGroupPathRepeatedNameIsALoop(t *testing.T) {
	base := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	steps := []reads.Step{
		{Name: "spin", State: status.Success, StartedAt: base, CompletedAt: base.Add(100 * time.Millisecond)},
		{Name: "spin", State: status.Success, StartedAt: base.Add(200 * time.Millisecond), CompletedAt: base.Add(300 * time.Millisecond)},
		{Name: "spin", State: status.Success, StartedAt: base.Add(400 * time.Millisecond), CompletedAt: base.Add(500 * time.Millisecond)},
	}
	g := groupPath(steps)
	if len(g) != 1 || g[0].Step != "spin ×3" {
		t.Fatalf("loop: want the single loop node, got %v", g)
	}
	svg := string(ExecutedPathToSVG(steps))
	if !strings.Contains(svg, `loop-symbol`) {
		t.Fatalf("loop: want the loop arc:\n%s", svg)
	}
	if !strings.Contains(svg, `>spin ×3</text>`) {
		t.Fatalf("loop: want the ×3 count:\n%s", svg)
	}
}

// TestGroupPathCollapsesLargeGroups: a group of more than 4 members
// collapses into one annotated box (×N); the full member list lives in the
// box's <title>.
func TestGroupPathCollapsesLargeGroups(t *testing.T) {
	base := time.Date(2026, 7, 15, 10, 0, 0, 0, time.UTC)
	names := []string{"a", "b", "c", "d", "e"}
	var steps []reads.Step
	for _, name := range names {
		steps = append(steps, reads.Step{Name: name, State: status.Success, StartedAt: base, CompletedAt: base.Add(500 * time.Millisecond)})
	}
	g := groupPath(steps)
	if len(g) != 1 || g[0].Step != "×5" {
		t.Fatalf("collapse: want the single collapsed node, got %v", g)
	}
	svg := string(ExecutedPathToSVG(steps))
	if n := strings.Count(svg, `<rect class="g-node`); n != 1 {
		t.Fatalf("collapse: got %d boxes, want 1:\n%s", n, svg)
	}
	if !strings.Contains(svg, `>×5</text>`) {
		t.Fatalf("collapse: want the ×5 annotation:\n%s", svg)
	}
	title := boxTitle(t, svg)
	for _, name := range names {
		if !strings.Contains(title, name) {
			t.Errorf("collapse: the <title> must list every member, missing %q: %q", name, title)
		}
	}
}

// TestGroupPathChildBoxCarriesTheChildSymbol: the child row renders the
// child box — the child-symbol (◉), the child's name, the child's live
// badge state.
func TestGroupPathChildBoxCarriesTheChildSymbol(t *testing.T) {
	steps := []reads.Step{
		{Name: "DBOS.enqueue", Kind: reads.KindChild, ChildID: "c1", ChildName: "buy", ChildState: status.Queued, State: status.Success},
	}
	svg := string(ExecutedPathToSVG(steps))
	if !strings.Contains(svg, `class="child-symbol"`) {
		t.Fatalf("child box: want the child symbol:\n%s", svg)
	}
	if !strings.Contains(svg, `>buy</text>`) {
		t.Fatalf("child box: want the child name:\n%s", svg)
	}
	if !strings.Contains(svg, `class="g-node queued"`) {
		t.Fatalf("child box: want the child's live badge:\n%s", svg)
	}
}

// TestGroupPathWaitBoxCarriesTheWaitSymbol: the wait row renders the wait
// symbol (… — the three dots).
func TestGroupPathWaitBoxCarriesTheWaitSymbol(t *testing.T) {
	steps := []reads.Step{
		{Name: "DBOS.select", Kind: reads.KindWait, State: status.Success},
	}
	svg := string(ExecutedPathToSVG(steps))
	if !strings.Contains(svg, `class="wait-symbol"`) {
		t.Fatalf("wait box: want the wait symbol:\n%s", svg)
	}
	if !strings.Contains(svg, `>DBOS.select</text>`) {
		t.Fatalf("wait box: want the row name:\n%s", svg)
	}
}

func boxTitle(t *testing.T, svg string) string {
	t.Helper()
	i := strings.Index(svg, `<title>`)
	if i < 0 {
		t.Fatalf("no <title> in:\n%s", svg)
	}
	i += len(`<title>`)
	j := strings.Index(svg[i:], `</title>`)
	if j < 0 {
		t.Fatalf("unterminated <title>: %s", svg)
	}
	return svg[i : i+j]
}
