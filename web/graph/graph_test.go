package graph

import (
	"encoding/xml"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/core/status"
)

// TestPathsGraphBuildsTheTree: the common prefix is drawn once and the if
// forks the children; a step where some path ends is terminal (every leaf,
// and a fork where one path ends early).
func TestPathsGraphBuildsTheTree(t *testing.T) {
	tests := []struct {
		name  string
		paths [][]string
		want  string
	}{
		{"no paths", nil, ""},
		{"single step", [][]string{{"greet"}}, "greet?"},
		{"linear chain", [][]string{{"a", "b", "c"}}, "a{b{c?}}"},
		{"if forks the children", [][]string{{"base", "left"}, {"base", "right"}}, "base{left?,right?}"},
		{"a path ends at the fork", [][]string{{"a"}, {"a", "b"}}, "a?{b?}"},
		{"two roots", [][]string{{"a"}, {"b"}}, "a?;b?"},
		{"empty paths are ignored", [][]string{{}, {"a"}}, "a?"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := treeString(pathsGraph(tt.paths)); got != tt.want {
				t.Errorf("tree: got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNewGraphLayoutLinearRow: a chain without an if runs in one row, left
// to right, with one edge per continuation.
func TestNewGraphLayoutLinearRow(t *testing.T) {
	g := newGraphLayout(pathsGraph([][]string{{"a", "b", "c"}}))
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
	g := newGraphLayout(pathsGraph([][]string{{"base", "left"}, {"base", "right"}}))
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
	g := newGraphLayout(pathsGraph([][]string{{"a", "b", "c"}, {"a", "b", "d"}, {"a", "e"}}))
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
	g := newGraphLayout(pathsGraph(nil))
	if len(g.nodes) != 0 || len(g.edges) != 0 {
		t.Fatalf("empty layout: got %d nodes / %d edges", len(g.nodes), len(g.edges))
	}
	if g.width <= 0 || g.height <= 0 {
		t.Errorf("empty canvas must be sized: %v x %v", g.width, g.height)
	}
}

// TestPathsToSVGEmitsNodesAndEdges: one rect per step, one line per
// continuation, and every edge carries the arrowhead marker.
func TestPathsToSVGEmitsNodesAndEdges(t *testing.T) {
	svg := string(PathsToSVG([][]string{{"a", "b"}, {"a", "c"}}))
	if n := strings.Count(svg, `<rect class="g-node"`); n != 3 {
		t.Errorf("rects: got %d, want 3", n)
	}
	if n := strings.Count(svg, `<line class="g-edge"`); n != 2 {
		t.Errorf("lines: got %d, want 2", n)
	}
	if !strings.Contains(svg, `marker-end="url(#flowdeck-arrow)"`) {
		t.Error("edges without the arrowhead marker")
	}
	for _, step := range []string{">a<", ">b<", ">c<"} {
		if !strings.Contains(svg, step) {
			t.Errorf("svg missing the node label %q", step)
		}
	}
}

// TestPathsToSVGEscapesStepNames: the node labels are escaped, not raw.
func TestPathsToSVGEscapesStepNames(t *testing.T) {
	svg := string(PathsToSVG([][]string{{"a<b>"}}))
	if !strings.Contains(svg, "a&lt;b&gt;") || strings.Contains(svg, "a<b>") {
		t.Errorf("step name not escaped: %q", svg)
	}
}

// TestPathsToSVGIsWellFormedXML: the generated SVG parses as XML.
func TestPathsToSVGIsWellFormedXML(t *testing.T) {
	for _, paths := range [][][]string{
		{nil},
		{{"a", "b", "c"}, {"a", "d"}, {"a", "e", "f"}},
	} {
		svg := string(PathsToSVG(paths))
		if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
			t.Errorf("not well-formed XML (%v): %v", paths, err)
		}
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

// treeString renders the paths tree compactly: steps in braces, a terminal
// step gets "?", and roots are separated by ";".
func treeString(roots []pathNode) string {
	var b strings.Builder
	for i, r := range roots {
		if i > 0 {
			b.WriteString(";")
		}
		writeNodeTree(&b, r)
	}
	return b.String()
}

func writeNodeTree(b *strings.Builder, n pathNode) {
	b.WriteString(n.Step)
	if n.Terminal {
		b.WriteString("?")
	}
	if len(n.Children) > 0 {
		b.WriteString("{")
		for i, c := range n.Children {
			if i > 0 {
				b.WriteString(",")
			}
			writeNodeTree(b, c)
		}
		b.WriteString("}")
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
