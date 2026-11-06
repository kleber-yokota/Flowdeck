// block_test.go asserts BlockToSVG renders the block structure: nil ·
// sequence · loop · forks · scale · async (side by side · group edge) · join.
package block

import (
	"encoding/xml"
	"math"
	"strconv"
	"strings"
	"testing"

	"flowdeck/core/graph"
)

func TestBlockToSVGNil(t *testing.T) {
	if got := string(BlockToSVG(nil)); got != "" {
		t.Fatalf("nil workflow: got %q, want empty", got)
	}
	if got := string(BlockToSVG(&graph.WorkflowGraph{})); got != "" {
		t.Fatalf("empty workflow (no root): got %q, want empty", got)
	}
}

// TestBlockToSVGSequence: three steps are three boxes, top to bottom, each
// arrow to the next, and the root sequence is the L0 outline.
func TestBlockToSVGSequence(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Node: &graph.Node{Name: "a", Kind: graph.NodeStep}},
			{Node: &graph.Node{Name: "b", Kind: graph.NodeStep}},
			{Node: &graph.Node{Name: "c", Kind: graph.NodeStep}},
		}},
	}))
	if !strings.Contains(svg, `>a</text>`) || !strings.Contains(svg, `>b</text>`) || !strings.Contains(svg, `>c</text>`) {
		t.Fatalf("sequence: want the three boxes:\n%s", svg)
	}
	aY, bY, cY := boxY(t, svg, "a"), boxY(t, svg, "b"), boxY(t, svg, "c")
	if !(aY < bY && bY < cY) {
		t.Fatalf("sequence: want a above b above c, got y a=%v b=%v c=%v", aY, bY, cY)
	}
	if n := strings.Count(svg, `<line class="g-edge"`); n != 2 {
		t.Fatalf("sequence arrows: got %d, want 2", n)
	}
	if !strings.Contains(svg, `class="g-block l0"`) {
		t.Fatalf("sequence: want the L0 outline:\n%s", svg)
	}
}

// TestBlockToSVGLoop: the loop block is its level outline, stroked at the
// loop's level, and it carries the loop symbol.
func TestBlockToSVGLoop(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockLoop, Children: []graph.Child{
				{Node: &graph.Node{Name: "b", Kind: graph.NodeStep}},
			}}},
		}},
	}))
	if !strings.Contains(svg, `class="g-block l1"`) {
		t.Fatalf("loop: want the L1 outline:\n%s", svg)
	}
	if !strings.Contains(svg, "loop-symbol") {
		t.Fatalf("loop: want the loop symbol:\n%s", svg)
	}
}

// TestBlockToSVGScale: the whole block-DAG svg is scaled 1.25× (readability)
// — the canvas is the content canvas × 1.25, everything inside the scale
// group stays in content coordinates.
func TestBlockToSVGScale(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockLoop, Children: []graph.Child{
				{Node: &graph.Node{Name: "spin", Kind: graph.NodeStep}},
			}}},
		}},
	}))
	if !strings.Contains(svg, `<g transform="scale(1.25)">`) {
		t.Fatalf("block svg: want the 1.25× scale group:\n%s", svg)
	}
	// The canvas of this loop graph is 144×120 → the svg is 180×150
	// (11px/char boxes, 48px tall — the readability geometry, fact 18).
	if !strings.Contains(svg, `width="180" height="150"`) {
		t.Fatalf("block svg: want the scaled canvas 167×135:\n%s", svg)
	}
	if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
}

// TestBlockToSVFForks: the if is the branches side by side (the if-body and
// the skip), each branch an arrow to the step after the if.
func TestBlockToSVFForks(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 0, Type: graph.BlockIf, Children: []graph.Child{
				{Node: &graph.Node{Name: "if", Kind: graph.NodeStep}},
			}}},
			{Node: &graph.Node{Name: "after", Kind: graph.NodeStep}},
		}},
	}))
	if !strings.Contains(svg, `>if</text>`) || !strings.Contains(svg, `>after</text>`) {
		t.Fatalf("if: want both branches and the step after:\n%s", svg)
	}
	// The if forks: the step after the if is reached by an arrow from the
	// branch (the skip branch has no box, so it has no arrow).
	if n := strings.Count(svg, `<line class="g-edge"`); n < 1 {
		t.Fatalf("if fork arrows: got %d, want >=1", n)
	}
}

// TestBlockToSVGAsyncGroupEdge: the async run is one logical unit — the
// single edge goes from the group's bottom-center to the next box's
// top-center (vertical), not from one box of the run.
func TestBlockToSVGAsyncGroupEdge(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Node: &graph.Node{Name: "alpha", Kind: graph.NodeAsync}},
			{Node: &graph.Node{Name: "beta", Kind: graph.NodeAsync}},
			{Node: &graph.Node{Name: "select", Kind: graph.NodeWait}},
		}},
	}))
	if n := strings.Count(svg, `<line class="g-edge"`); n != 1 {
		t.Fatalf("async group: want exactly one edge (group → wait), got %d:\n%s", n, svg)
	}
	idx := strings.Index(svg, `<line class="g-edge"`)
	end := strings.Index(svg[idx:], `>`)
	x1, _ := strconv.ParseFloat(attrValue(svg[idx:idx+end], "x1"), 64)
	x2, _ := strconv.ParseFloat(attrValue(svg[idx:idx+end], "x2"), 64)
	if math.Abs(x1-x2) > 0.01 {
		t.Fatalf("async group edge: want vertical (group center → wait center), got x1=%v x2=%v", x1, x2)
	}
}

// TestBlockToSVGAsyncSideBySide: the async nodes are side by side (same row,
// no arrow between them).
func TestBlockToSVGAsyncSideBySide(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Node: &graph.Node{Name: "a", Kind: graph.NodeAsync}},
			{Node: &graph.Node{Name: "b", Kind: graph.NodeAsync}},
		}},
	}))
	aY, bY := boxY(t, svg, "a"), boxY(t, svg, "b")
	if aY != bY {
		t.Fatalf("async: want the two boxes on the same row, got y a=%v b=%v", aY, bY)
	}
	if n := strings.Count(svg, `<line class="g-edge"`); n != 0 {
		t.Fatalf("async: want no arrow between them, got %d", n)
	}
}

// translateOf is the translate(x,y) of a transformed element tag.
// The join block is one outline (at its level) around the async boxes and
// the wait that joins them.
func TestBlockToSVGJoinOutline(t *testing.T) {
	wg := &graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockJoin, Children: []graph.Child{
				{Node: &graph.Node{Name: "alpha", Kind: graph.NodeAsync}},
				{Node: &graph.Node{Name: "beta", Kind: graph.NodeAsync}},
				{Node: &graph.Node{Name: "select", Kind: graph.NodeWait}},
			}}},
		}},
	}
	svg := string(BlockToSVG(wg))
	if n := strings.Count(svg, `<rect class="g-block`); n != 2 {
		t.Fatalf("outlines: got %d, want 2 (the L0 root and the L1 join):\n%s", n, svg)
	}
	l1y, l1h := outlineY(t, svg, "l1")
	selY, ok := boxTextY(t, svg, "select")
	if !ok {
		t.Fatalf("want the select box:\n%s", svg)
	}
	boxY := selY - 29 // the label is box.y + 24 + 5
	if boxY < l1y || boxY+48 > l1y+l1h {
		t.Fatalf("want the select box inside the L1 outline (y %v..%v), got box y %v", l1y, l1y+l1h, boxY)
	}
	if !strings.Contains(svg, `class="async-symbol"`) {
		t.Fatalf("want the ∥ mark inside the join:\n%s", svg)
	}
}

// boxRect is the x/y/w of the box whose label is text.
func boxRect(t *testing.T, svg, text string) (x, y, w float64) {
	t.Helper()
	idx := strings.Index(svg, `>`+text+`</text>`)
	if idx < 0 {
		t.Fatalf("label %q not found", text)
	}
	rect := strings.LastIndex(svg[:idx], `<rect`)
	if rect < 0 {
		t.Fatalf("no rect before label %q", text)
	}
	end := strings.Index(svg[rect:], `>`)
	if end < 0 {
		t.Fatalf("unterminated rect tag before %q", text)
	}
	tag := svg[rect : rect+end]
	x, _ = strconv.ParseFloat(attrValue(tag, "x"), 64)
	y, _ = strconv.ParseFloat(attrValue(tag, "y"), 64)
	w, _ = strconv.ParseFloat(attrValue(tag, "width"), 64)
	return x, y, w
}

// boxY is the y (top) of the box whose label is text.
func boxY(t *testing.T, svg, text string) float64 {
	_, y, _ := boxRect(t, svg, text)
	return y
}

func boxTextY(t *testing.T, svg, label string) (ty float64, ok bool) {
	t.Helper()
	i := strings.Index(svg, ">"+label+"</text>")
	if i < 0 {
		return 0, false
	}
	start := strings.LastIndex(svg[:i], "<text ")
	if start < 0 {
		return 0, false
	}
	ty, err := strconv.ParseFloat(attrValue(svg[start:i], "y"), 64)
	if err != nil {
		return 0, false
	}
	return ty, true
}

// attrValue is the value of an attribute in an XML tag string.
func attrValue(tag, name string) string {
	for _, part := range strings.Split(tag, " ") {
		if strings.HasPrefix(part, name+"=") {
			return strings.Trim(strings.TrimPrefix(part, name+"="), `"`)
		}
	}
	return ""
}
