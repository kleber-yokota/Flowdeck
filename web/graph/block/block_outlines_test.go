// block_outlines_test.go asserts the block outlines: the root outline
// clears the inner blocks, and no two outlines overlap.
package block

import (
	"strconv"
	"strings"
	"testing"

	"flowdeck/core/graph"
)

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
