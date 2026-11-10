package executed

import (
	"encoding/xml"
	"strconv"
	"strings"
	"testing"

	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// TestExecutedPathToSVGChildNodeShowsChildNameAndBadge: the child row
// (a RunWorkflow dispatch) renders as the child workflow — the node is
// labeled with the child's name and colored with the child's live badge
// state (the row's own state is the dispatch's, not the child's).
func TestExecutedPathToSVGChildNodeShowsChildNameAndBadge(t *testing.T) {
	steps := []reads.Step{
		{Name: "DBOS.enqueue", Kind: reads.KindChild, ChildID: "child-1", ChildName: "pipeline", ChildState: status.Running, State: status.Success},
		{Name: "DBOS.getResult", Kind: reads.KindWait, State: status.Success},
	}
	svg := string(ExecutedPathToSVG(steps, false))
	if !strings.Contains(svg, `class="g-node running"`) {
		t.Errorf("child node missing the child's live state: %s", svg)
	}
	if !strings.Contains(svg, ">pipeline<") {
		t.Errorf("child node missing the child's name: %s", svg)
	}
	if !strings.Contains(svg, `class="g-node success"`) {
		t.Errorf("wait node missing its own state: %s", svg)
	}
	if !strings.Contains(svg, ">DBOS.getResult<") {
		t.Errorf("wait node missing the row name: %s", svg)
	}
}

// TestExecutedPathToSVGChildFallbackUsesRowName: a child row missing the
// child's name (the child row is absent — defensive) renders the row name
// and the row's own state.
func TestExecutedPathToSVGChildFallbackUsesRowName(t *testing.T) {
	steps := []reads.Step{
		{Name: "DBOS.enqueue", Kind: reads.KindChild, ChildID: "child-1", State: status.Success},
	}
	svg := string(ExecutedPathToSVG(steps, false))
	if !strings.Contains(svg, `class="g-node success"`) {
		t.Errorf("fallback node missing the row's state: %s", svg)
	}
	if !strings.Contains(svg, ">DBOS.enqueue<") {
		t.Errorf("fallback node missing the row name: %s", svg)
	}
}

// TestNewGraphLayoutLinearRow: the linear path runs top to bottom — one
// box per depth (y increases), all in the same column (x).
func TestNewGraphLayoutLinearRow(t *testing.T) {
	g := newGraphLayout([]pathNode{{Step: "a", Children: []pathNode{{Step: "b", Children: []pathNode{{Step: "c"}}}}}})
	if len(g.nodes) != 3 || len(g.edges) != 2 {
		t.Errorf("nodes/edges: got %d/%d, want 3/2", len(g.nodes), len(g.edges))
	}
	col := g.nodes[0].x
	for i, n := range g.nodes {
		if n.x != col {
			t.Errorf("node %d on another column: x=%v want %v", i, n.x, col)
		}
		if i > 0 && n.y <= g.nodes[i-1].y {
			t.Errorf("node %d not below node %d: y=%v vs %v", i, i-1, n.y, g.nodes[i-1].y)
		}
	}
}

// TestNewGraphLayoutForks: the fork puts the children on distinct columns
// below the parent, and the parent centers on its children's x.
func TestNewGraphLayoutForks(t *testing.T) {
	g := newGraphLayout([]pathNode{{Step: "base", Children: []pathNode{{Step: "left"}, {Step: "right"}}}})
	if len(g.nodes) != 3 || len(g.edges) != 2 {
		t.Errorf("nodes/edges: got %d/%d, want 3/2", len(g.nodes), len(g.edges))
	}
	base, ok := findNode(g, "base")
	if !ok {
		t.Error("layout missing the base node")
	}
	left, ok := findNode(g, "left")
	if !ok {
		t.Error("layout missing the left node")
	}
	right, ok := findNode(g, "right")
	if !ok {
		t.Error("layout missing the right node")
	}
	if base.x != (left.x+right.x)/2 {
		t.Errorf("parent not centered: x=%v want %v", base.x, (left.x+right.x)/2)
	}
	if left.y <= base.y {
		t.Error("fork children must be below the parent (left.y > base.y)")
	}
	if n := distinctCols(g); n != 3 {
		t.Errorf("columns: got %d, want 3 (base, left, right)", n)
	}
}

// TestNewGraphLayoutNestedFork: a fork inside a fork keeps the columns
// tidy — every node centers on its children's x.
func TestNewGraphLayoutNestedFork(t *testing.T) {
	g := newGraphLayout([]pathNode{{Step: "a", Children: []pathNode{{Step: "b", Children: []pathNode{{Step: "c"}, {Step: "d"}}}, {Step: "e"}}}})
	a, ok := findNode(g, "a")
	if !ok {
		t.Error("layout missing the a node")
	}
	b, ok := findNode(g, "b")
	if !ok {
		t.Error("layout missing the b node")
	}
	c, ok := findNode(g, "c")
	if !ok {
		t.Error("layout missing the c node")
	}
	d, ok := findNode(g, "d")
	if !ok {
		t.Error("layout missing the d node")
	}
	e, ok := findNode(g, "e")
	if !ok {
		t.Error("layout missing the e node")
	}
	if b.x != (c.x+d.x)/2 {
		t.Errorf("b not centered on c/d: x=%v want %v", b.x, (c.x+d.x)/2)
	}
	if a.x != (b.x+e.x)/2 {
		t.Errorf("a not centered on b/e: x=%v want %v", a.x, (b.x+e.x)/2)
	}
}

// TestNewGraphLayoutEmpty: no paths is a valid empty canvas (no nodes,
// still a sized svg).
func TestNewGraphLayoutEmpty(t *testing.T) {
	g := newGraphLayout(nil)
	if len(g.nodes) != 0 || len(g.edges) != 0 {
		t.Errorf("empty layout: got %d nodes / %d edges", len(g.nodes), len(g.edges))
	}
	if g.width <= 0 || g.height <= 0 {
		t.Errorf("empty canvas must be sized: %v x %v", g.width, g.height)
	}
}

// TestExecutedPathToSVGLinearChain: the executed path is the same graph,
// with the single row of the steps that ran — one node per step, one edge
// per continuation, and every node colored by the step's state.
func TestExecutedPathToSVGLinearChain(t *testing.T) {
	steps := []reads.Step{
		{Name: "fetch", State: status.Success},
		{Name: "parse", State: status.Success},
		{Name: "store", State: status.Success},
	}
	svg := string(ExecutedPathToSVG(steps, false))
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
	steps := []reads.Step{{Name: "fetch", State: status.Success}}
	svg := string(ExecutedPathToSVG(steps, false))
	if !strings.Contains(svg, `<g transform="scale(1.25)">`) {
		t.Errorf("executed path svg: want the 1.25× scale group:\n%s", svg)
	}
	if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
		t.Errorf("not well-formed XML: %v", err)
	}
}

// TestExecutedPathTailWhenLive: a live run (the steps that ran so far — no
// failed row, the run still running) gets the dashed tail box after the
// last box: the only "in progress" marker in the graph. The tail consumes
// one depth row (the viewBox height grows by one row).
func TestExecutedPathTailWhenLive(t *testing.T) {
	steps := []reads.Step{
		{Name: "fetch", State: status.Success},
		{Name: "parse", State: status.Success},
		{Name: "store", State: status.Success},
	}
	svg := string(ExecutedPathToSVG(steps, true))
	if !strings.Contains(svg, `<rect class="g-node tail"`) {
		t.Errorf("live path missing the tail rect:\n%s", svg)
	}
	if !strings.Contains(svg, `class="g-text tail"`) {
		t.Errorf("live path missing the tail text class:\n%s", svg)
	}
	if !strings.Contains(svg, `>…</text>`) {
		t.Errorf("live path missing the tail label:\n%s", svg)
	}
	h, hPlain := viewBoxH(t, svg), viewBoxH(t, string(ExecutedPathToSVG(steps, false)))
	if h != hPlain+90 {
		t.Errorf("tail row: viewBox height %d, want %d (one depth row at 1.25x)", h, hPlain+90)
	}
}

// TestExecutedPathNoTailWhenTerminal: a terminal run has no tail box (the
// existing terminal-path output is unchanged).
func TestExecutedPathNoTailWhenTerminal(t *testing.T) {
	steps := []reads.Step{
		{Name: "fetch", State: status.Success},
		{Name: "parse", State: status.Success},
		{Name: "store", State: status.Success},
	}
	svg := string(ExecutedPathToSVG(steps, false))
	if strings.Contains(svg, "g-node tail") {
		t.Errorf("terminal path must not have the tail:\n%s", svg)
	}
	if strings.Contains(svg, "g-text tail") {
		t.Errorf("terminal path must not have the tail text:\n%s", svg)
	}
}

// viewBoxH: the svg's viewBox height (the 1.25x-scaled canvas height).
func viewBoxH(t *testing.T, svg string) int {
	t.Helper()
	i := strings.Index(svg, `viewBox="0 0 `)
	if i < 0 {
		t.Errorf("svg missing the viewBox:\n%s", svg)
	}
	h := svg[i+len(`viewBox="0 0 `):]
	h = h[strings.IndexByte(h, ' ')+1:]
	h = h[:strings.IndexByte(h, '"')]
	n, err := strconv.Atoi(h)
	if err != nil {
		t.Errorf("bad viewBox height %q: %v", h, err)
	}
	return n
}

// TestExecutedPathToSVGFailedStep: a failed step is colored with the
// failed state (the rest of the path keeps the success color).
func TestExecutedPathToSVGFailedStep(t *testing.T) {
	steps := []reads.Step{
		{Name: "fetch", State: status.Success},
		{Name: "parse", State: status.Error},
	}
	svg := string(ExecutedPathToSVG(steps, false))
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
		t.Errorf("empty layout: got %d nodes / %d edges", len(g.nodes), len(g.edges))
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

func distinctCols(g *graphLayout) int {
	xs := map[float64]bool{}
	for _, n := range g.nodes {
		xs[n.x] = true
	}
	return len(xs)
}
