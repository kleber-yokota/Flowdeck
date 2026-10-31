package graph

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
	// The canvas of this loop graph is 133.6×108 → the svg is 167×135.
	if !strings.Contains(svg, `width="167" height="135"`) {
		t.Fatalf("block svg: want the scaled canvas 167×135:\n%s", svg)
	}
	if err := xml.Unmarshal([]byte(svg), new(struct{})); err != nil {
		t.Fatalf("not well-formed XML: %v", err)
	}
}

// TestBlockToSVGLoopSymbolShape: the loop symbol is the open 300° arc with an
// arrowhead (not a plain circle).
func TestBlockToSVGLoopSymbolShape(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockLoop, Children: []graph.Child{
				{Node: &graph.Node{Name: "b", Kind: graph.NodeStep}},
			}}},
		}},
	}))
	sy := strings.Index(svg, `class="loop-symbol"`)
	if sy < 0 {
		t.Fatalf("loop: want the loop symbol:\n%s", svg)
	}
	sEnd := strings.Index(svg[sy:], `</g>`)
	sym := svg[sy : sy+sEnd]
	if strings.Contains(sym, `<circle`) {
		t.Fatalf("loop symbol: want the arc + arrowhead, got a circle: %s", sym)
	}
	if !strings.Contains(sym, `M5.5,0 A5.5,5.5 0 1 1 2.75,-4.76`) {
		t.Fatalf("loop symbol: want the 300° arc: %s", sym)
	}
	if !strings.Contains(sym, `<polygon`) {
		t.Fatalf("loop symbol: want the arrowhead polygon: %s", sym)
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

// TestBlockToSVGNodeSymbols: the child and wait nodes carry their symbols
// (the box is drawn for every kind).
func TestBlockToSVGNodeSymbols(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Node: &graph.Node{Name: "child", Kind: graph.NodeChild}},
			{Node: &graph.Node{Name: "wait", Kind: graph.NodeWait}},
		}},
	}))
	if !strings.Contains(svg, "child-ring") || !strings.Contains(svg, "child-dot") {
		t.Fatalf("child: want the child symbol (ring + dot):\n%s", svg)
	}
	if !strings.Contains(svg, `>child</text>`) {
		t.Fatalf("child: want the box:\n%s", svg)
	}
	if !strings.Contains(svg, "wait-symbol") || !strings.Contains(svg, `>wait</text>`) {
		t.Fatalf("wait: want the box and the wait symbol:\n%s", svg)
	}
}

// TestBlockToSVGNodeSymbolSpace: the child/wait boxes reserve the symbol
// zone on the right — the label is centered in the remaining space, so the
// symbol never overlaps the word.
func TestBlockToSVGNodeSymbolSpace(t *testing.T) {
	for _, kind := range []graph.NodeKind{graph.NodeChild, graph.NodeWait} {
		svg := string(BlockToSVG(&graph.WorkflowGraph{
			Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
				{Node: &graph.Node{Name: "select", Kind: kind}},
			}},
		}))
		boxX, _, boxW := boxRect(t, svg, "select")
		labelX := labelXOf(t, svg, "select")
		if labelX >= boxX+boxW/2 {
			t.Fatalf("%s: the label is not shifted left of the box center (no symbol zone): label x=%v, box center=%v", kind, labelX, boxX+boxW/2)
		}
	}
}

// TestBlockToSVGAsyncParallelMark: the async boxes are side by side with the
// parallel (∥) mark between them.
func TestBlockToSVGAsyncParallelMark(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Node: &graph.Node{Name: "a", Kind: graph.NodeAsync}},
			{Node: &graph.Node{Name: "b", Kind: graph.NodeAsync}},
		}},
	}))
	if !strings.Contains(svg, "async-symbol") {
		t.Fatalf("async: want the parallel mark between the boxes:\n%s", svg)
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

// TestBlockToSVGLoopSymbolTopRight: the loop symbol is anchored at the
// top-right corner of the loop outline (consistent with the right-side
// node-kind symbols).
func TestBlockToSVGLoopSymbolTopRight(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Node: &graph.Node{Name: "a", Kind: graph.NodeStep}},
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockLoop, Children: []graph.Child{
				{Node: &graph.Node{Name: "spin", Kind: graph.NodeStep}},
			}}},
		}},
	}))
	oi := strings.Index(svg, `<rect class="g-block l1"`)
	if oi < 0 {
		t.Fatalf("loop: want the L1 outline:\n%s", svg)
	}
	oEnd := strings.Index(svg[oi:], `>`)
	outlineX, _ := strconv.ParseFloat(attrValue(svg[oi:oi+oEnd], "x"), 64)
	outlineY, _ := strconv.ParseFloat(attrValue(svg[oi:oi+oEnd], "y"), 64)
	outlineW, _ := strconv.ParseFloat(attrValue(svg[oi:oi+oEnd], "width"), 64)
	outlineH, _ := strconv.ParseFloat(attrValue(svg[oi:oi+oEnd], "height"), 64)
	sy := strings.Index(svg, `class="loop-symbol"`)
	if sy < 0 {
		t.Fatalf("loop: want the loop symbol:\n%s", svg)
	}
	sEnd := strings.Index(svg[sy:], `>`)
	tx, ty := translateOf(t, svg[sy:sy+sEnd])
	if tx < outlineX+outlineW/2 || ty > outlineY+outlineH/2 {
		t.Fatalf("loop symbol: want the top-right corner of the outline, got translate(%v,%v) for outline x=%v y=%v w=%v h=%v", tx, ty, outlineX, outlineY, outlineW, outlineH)
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

// labelXOf is the x (center) of the label text.
func labelXOf(t *testing.T, svg, text string) float64 {
	t.Helper()
	idx := strings.Index(svg, `>`+text+`</text>`)
	if idx < 0 {
		t.Fatalf("label %q not found", text)
	}
	start := strings.LastIndex(svg[:idx], `<text`)
	if start < 0 {
		t.Fatalf("no text tag before label %q", text)
	}
	ag := strings.Index(svg[start:], `>`)
	if ag < 0 {
		t.Fatalf("unterminated text tag before %q", text)
	}
	return mustFloat(t, attrValue(svg[start:start+ag], "x"))
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
	boxY := selY - 23 // the label is box.y + 18 + 5
	if boxY < l1y || boxY+36 > l1y+l1h {
		t.Fatalf("want the select box inside the L1 outline (y %v..%v), got box y %v", l1y, l1y+l1h, boxY)
	}
	if !strings.Contains(svg, `class="async-symbol"`) {
		t.Fatalf("want the ∥ mark inside the join:\n%s", svg)
	}
}

// The L0 (outermost) outline is a bit larger than the inner outlines, so it
// never shares an edge with a block that fills the canvas (a trailing loop).
func TestBlockToSVGRootOutlineClearsInnerBlocks(t *testing.T) {
	svg := string(BlockToSVG(&graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockLoop, Children: []graph.Child{
				{Node: &graph.Node{Name: "spin", Kind: graph.NodeStep}},
			}}},
		}},
	}))
	l0x, l0y, l0w, l0h := outlineRect(t, svg, "l0")
	l1x, l1y, l1w, l1h := outlineRect(t, svg, "l1")
	if !(l0x < l1x && l0y < l1y && l0x+l0w > l1x+l1w && l0y+l0h > l1y+l1h) {
		t.Fatalf("want the L0 outline strictly outside the L1: l0=(%v,%v,%v,%v) l1=(%v,%v,%v,%v)",
			l0x, l0y, l0w, l0h, l1x, l1y, l1w, l1h)
	}
}

// No two block outlines overlap (a block right after a block: the row gap is
// measured from the outlines, not from the content).
func TestBlockToSVGOutlinesDontOverlap(t *testing.T) {
	wg := &graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 1, Type: graph.BlockJoin, Children: []graph.Child{
				{Node: &graph.Node{Name: "side", Kind: graph.NodeAsync}},
				{Node: &graph.Node{Name: "select", Kind: graph.NodeWait}},
			}}},
			{Block: &graph.Block{ID: "b2", Level: 1, Type: graph.BlockLoop, Children: []graph.Child{
				{Node: &graph.Node{Name: "spin", Kind: graph.NodeStep}},
			}}},
		}},
	}
	svg := string(BlockToSVG(wg))
	rects := outlineRects(t, svg)
	for i := 0; i < len(rects); i++ {
		for j := i + 1; j < len(rects); j++ {
			a, b := rects[i], rects[j]
			intersects := a.x < b.x+b.w && b.x < a.x+a.w && a.y < b.y+b.h && b.y < a.y+a.h
			if intersects && !contains(a, b) && !contains(b, a) {
				t.Fatalf("outlines %d and %d overlap: %+v and %+v", i, j, a, b)
			}
		}
	}
}

func contains(a, b blockTestRect) bool {
	return a.x <= b.x && a.y <= b.y && a.x+a.w >= b.x+b.w && a.y+a.h >= b.y+b.h
}

type blockTestRect struct{ x, y, w, h float64 }

func outlineRects(t *testing.T, svg string) []blockTestRect {
	t.Helper()
	var rects []blockTestRect
	rest := svg
	for {
		i := strings.Index(rest, `<rect class="g-block `)
		if i < 0 {
			break
		}
		tagEnd := strings.IndexByte(rest[i:], '>')
		tag := rest[i : i+tagEnd]
		rects = append(rects, blockTestRect{
			x: parseAttr(t, tag, "x"), y: parseAttr(t, tag, "y"),
			w: parseAttr(t, tag, "width"), h: parseAttr(t, tag, "height"),
		})
		rest = rest[i+tagEnd:]
	}
	return rects
}

func parseAttr(t *testing.T, tag, name string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(attrValue(tag, name), 64)
	if err != nil {
		t.Fatalf("parse the %s attribute: %v", name, err)
	}
	return v
}

func outlineY(t *testing.T, svg, level string) (y, h float64) {
	_, y, _, h = outlineRect(t, svg, level)
	return
}

func outlineRect(t *testing.T, svg, level string) (x, y, w, h float64) {
	t.Helper()
	i := strings.Index(svg, `<rect class="g-block `+level+`"`)
	if i < 0 {
		t.Fatalf("want the %s outline:\n%s", level, svg)
	}
	end := strings.IndexByte(svg[i:], '>')
	tag := svg[i : i+end]
	var err error
	if x, err = strconv.ParseFloat(attrValue(tag, "x"), 64); err != nil {
		t.Fatalf("parse the %s outline x: %v", level, err)
	}
	if y, err = strconv.ParseFloat(attrValue(tag, "y"), 64); err != nil {
		t.Fatalf("parse the %s outline y: %v", level, err)
	}
	if w, err = strconv.ParseFloat(attrValue(tag, "width"), 64); err != nil {
		t.Fatalf("parse the %s outline width: %v", level, err)
	}
	if h, err = strconv.ParseFloat(attrValue(tag, "height"), 64); err != nil {
		t.Fatalf("parse the %s outline height: %v", level, err)
	}
	return
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

func translateOf(t *testing.T, tag string) (tx, ty float64) {
	t.Helper()
	inner := strings.TrimSuffix(strings.TrimPrefix(attrValue(tag, "transform"), "translate("), ")")
	parts := strings.Split(inner, ",")
	if len(parts) != 2 {
		t.Fatalf("bad translate: %q", attrValue(tag, "transform"))
	}
	tx = mustFloat(t, strings.TrimSpace(parts[0]))
	ty = mustFloat(t, strings.TrimSpace(parts[1]))
	return tx, ty
}

func mustFloat(t *testing.T, s string) float64 {
	t.Helper()
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return f
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
