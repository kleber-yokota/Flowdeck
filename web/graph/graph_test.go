package graph

import (
	"encoding/xml"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/core/status"
)

// TestNewGraphLayoutLinearRow: a chain without an if runs in one row, left
// to right, with one edge per continuation.
func TestNewGraphLayoutLinearRow(t *testing.T) {
	g := newGraphLayout([]pathNode{{Step: "a", Children: []pathNode{{Step: "b", Children: []pathNode{{Step: "c"}}}}}})
	if len(g.nodes) != 3 || len(g.edges) != 2 {
		t.Fatalf("nodes/edges: got %d/%d, want 3/2", len(g.nodes), len(g.edges))
	}
	row := g.nodes[0].y
	for i, n := range g.nodes {
		if n.y != row {
			t.Errorf("node %d on another row: y=%v want %v", i, n.y, row)
		}
		if i > 0 && n.x <= g.nodes[i-1].x {
			t.Errorf("node %d not to the right of node %d: x=%v vs %v", i, i-1, n.x, g.nodes[i-1].x)
		}
	}
}

// TestNewGraphLayoutForks: the if forks the branches on distinct rows and
// the parent centers on its children.
func TestNewGraphLayoutForks(t *testing.T) {
	g := newGraphLayout([]pathNode{{Step: "base", Children: []pathNode{{Step: "left"}, {Step: "right"}}}})
	if len(g.nodes) != 3 || len(g.edges) != 2 {
		t.Fatalf("nodes/edges: got %d/%d, want 3/2", len(g.nodes), len(g.edges))
	}
	base, ok := findNode(g, "base")
	if !ok {
		t.Fatal("layout missing the base node")
	}
	left, ok := findNode(g, "left")
	if !ok {
		t.Fatal("layout missing the left node")
	}
	right, ok := findNode(g, "right")
	if !ok {
		t.Fatal("layout missing the right node")
	}
	if base.y != (left.y+right.y)/2 {
		t.Errorf("parent not centered: y=%v want %v", base.y, (left.y+right.y)/2)
	}
	if left.x <= base.x || right.x <= base.x {
		t.Error("fork children must be to the right of the parent")
	}
	if n := distinctRows(g); n != 3 {
		t.Errorf("rows: got %d, want 3 (base, left, right)", n)
	}
}

// TestNewGraphLayoutNestedFork: a fork inside a fork keeps the rows tidy —
// every node centers on its children.
func TestNewGraphLayoutNestedFork(t *testing.T) {
	g := newGraphLayout([]pathNode{{Step: "a", Children: []pathNode{{Step: "b", Children: []pathNode{{Step: "c"}, {Step: "d"}}}, {Step: "e"}}}})
	a, ok := findNode(g, "a")
	if !ok {
		t.Fatal("layout missing the a node")
	}
	b, ok := findNode(g, "b")
	if !ok {
		t.Fatal("layout missing the b node")
	}
	c, ok := findNode(g, "c")
	if !ok {
		t.Fatal("layout missing the c node")
	}
	d, ok := findNode(g, "d")
	if !ok {
		t.Fatal("layout missing the d node")
	}
	e, ok := findNode(g, "e")
	if !ok {
		t.Fatal("layout missing the e node")
	}
	if b.y != (c.y+d.y)/2 {
		t.Errorf("b not centered on c/d: y=%v want %v", b.y, (c.y+d.y)/2)
	}
	if a.y != (b.y+e.y)/2 {
		t.Errorf("a not centered on b/e: y=%v want %v", a.y, (b.y+e.y)/2)
	}
}

// TestNewGraphLayoutEmpty: no paths is a valid empty canvas (no nodes,
// still a sized svg).
func TestNewGraphLayoutEmpty(t *testing.T) {
	g := newGraphLayout(nil)
	if len(g.nodes) != 0 || len(g.edges) != 0 {
		t.Fatalf("empty layout: got %d nodes / %d edges", len(g.nodes), len(g.edges))
	}
	if g.width <= 0 || g.height <= 0 {
		t.Errorf("empty canvas must be sized: %v x %v", g.width, g.height)
	}
}

// TestExecutedPathToSVGLinearChain: the executed path is the same graph,
// with the single row of the steps that ran — one node per step, one edge
// per continuation, and every node colored by the step's state.
func TestExecutedPathToSVGLinearChain(t *testing.T) {
	steps := []core.Step{
		{Name: "fetch", State: status.Success},
		{Name: "parse", State: status.Success},
		{Name: "store", State: status.Success},
	}
	svg := string(ExecutedPathToSVG(steps))
	if n := strings.Count(svg, `<rect class="g-node`); n != 3 {
		t.Errorf("rects: got %d, want 3", n)
	}
	if n := strings.Count(svg, `<line class="g-edge"`); n != 2 {
		t.Errorf("lines: got %d, want 2", n)
	}
	for _, want := range []string{`class="g-node success"`, `>fetch<`, `>parse<`, `>store<`, `aria-label="executed path"`} {
		if !strings.Contains(svg, want) {
			t.Errorf("svg does not contain %q", want)
		}
	}
	if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
		t.Errorf("not well-formed XML: %v", err)
	}
}

// TestExecutedPathToSVGScale: the executed-path svg is scaled 1.25× too
// (same readability scale as the block-DAG svg).
func TestExecutedPathToSVGScale(t *testing.T) {
	steps := []core.Step{{Name: "fetch", State: status.Success}}
	svg := string(ExecutedPathToSVG(steps))
	if !strings.Contains(svg, `<g transform="scale(1.25)">`) {
		t.Fatalf("executed path svg: want the 1.25× scale group:\n%s", svg)
	}
	if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
}

// TestExecutedPathToSVGFailedStep: a failed step is colored with the
// failed state (the rest of the path keeps the success color).
func TestExecutedPathToSVGFailedStep(t *testing.T) {
	steps := []core.Step{
		{Name: "fetch", State: status.Success},
		{Name: "parse", State: status.Error},
	}
	svg := string(ExecutedPathToSVG(steps))
	if !strings.Contains(svg, `class="g-node success"`) {
		t.Errorf("svg missing the successful node: %q", svg)
	}
	if !strings.Contains(svg, `class="g-node error"`) {
		t.Errorf("svg missing the failed node: %q", svg)
	}
}

// TestExecutedPathToSVGEmpty: a run with no steps yet is a valid empty
// canvas (no nodes, still a sized svg).
func TestExecutedPathToSVGEmpty(t *testing.T) {
	g := newGraphLayout(nil)
	if len(g.nodes) != 0 || len(g.edges) != 0 {
		t.Fatalf("empty layout: got %d nodes / %d edges", len(g.nodes), len(g.edges))
	}
	if g.width <= 0 || g.height <= 0 {
		t.Errorf("empty canvas must be sized: %v x %v", g.width, g.height)
	}
}

func findNode(g *graphLayout, step string) (graphNode, bool) {
	for _, n := range g.nodes {
		if n.step == step {
			return n, true
		}
	}
	return graphNode{}, false
}

func distinctRows(g *graphLayout) int {
	ys := map[float64]bool{}
	for _, n := range g.nodes {
		ys[n.y] = true
	}
	return len(ys)
}
