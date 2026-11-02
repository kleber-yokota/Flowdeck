// block_symbols_test.go asserts the node-kind symbols: the loop arc (shape ·
// top-right anchor), the child ring + dot, the wait dots, the async ∥ mark,
// and the symbol space (the label clears the symbol).
package graph

import (
	"strconv"
	"strings"
	"testing"

	"flowdeck/core/graph"
)

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
