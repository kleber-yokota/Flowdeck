// Package graph renders the workflow graphs as SVG: the executed path
// (a row of steps) and the possible paths (the block-DAG from
// core/graph.WorkflowGraph, no execution data).
//
// The block layout (block.go · block_fork.go) measures and places the
// boxes, outlines, and edges; block_svg.go emits the SVG. The geometry is
// a function of the graph alone (the constants are shared with the
// executed path in graph.go, so the two graphs read as one family).
package graph

import (
	"html/template"

	"flowdeck/core/graph"
)

// BlockToSVG renders the block tree (the possible-paths model) as the
// block-DAG SVG. A nil tree (or an empty root) is the empty SVG.
func BlockToSVG(wg *graph.WorkflowGraph) template.HTML {
	if wg == nil || wg.Root == nil {
		return template.HTML("")
	}
	l := &blockLayout{}
	rootBB, rootFirst, rootLast := l.layoutColumn(wg.Root.Children, 0, graphMargin, true)
	_ = rootFirst
	_ = rootLast
	// The root outline (the L0 sequence), a bit larger than the inner
	// outlines so it never shares an edge with a block that fills the
	// canvas (a trailing loop).
	l.outlines = append(l.outlines, outlinePadded(rootBB, 0, false, graphRootPad))
	l.canvas()
	return template.HTML(l.svg())
}

// blockBox is a positioned node box.
type blockBox struct {
	x, y, w, h float64
	text       string
	kind       graph.NodeKind
}

// blockOutline is a positioned block outline (stroked at the level color).
type blockOutline struct {
	x, y, w, h float64
	level      int
	isLoop     bool
}

// blockEdge is a positioned arrow.
type blockEdge struct {
	x1, y1, x2, y2 float64
}

// blockLayout is the measured block-DAG: the boxes, the outlines, the
// edges, the canvas, and the main-flow cursor (prev/forkPending).
type blockLayout struct {
	boxes       []blockBox
	outlines    []blockOutline
	edges       []blockEdge
	asyncMarks  []point // the ∥ marks between the boxes of an async run
	prev        *blockBox
	forkPending []*blockBox // the boxes that need an outgoing edge to the next box
	width       float64
	height      float64
}

type point struct{ x, y float64 }

// bbox is a bounding box.
type bbox struct {
	has                    bool
	minX, minY, maxX, maxY float64
}

func (b *bbox) add(x1, y1, x2, y2 float64) {
	if !b.has {
		b.minX, b.minY, b.maxX, b.maxY = x1, y1, x2, y2
		b.has = true
		return
	}
	if x1 < b.minX {
		b.minX = x1
	}
	if y1 < b.minY {
		b.minY = y1
	}
	if x2 > b.maxX {
		b.maxX = x2
	}
	if y2 > b.maxY {
		b.maxY = y2
	}
}

func (b *bbox) fold(o *bbox) {
	if o.has {
		b.add(o.minX, o.minY, o.maxX, o.maxY)
	}
}

// layoutColumn lays out the children (a vertical stack) centered at cx,
// starting at top. When connect is true, each child connects to the
// main-flow cursor (prev/forkPending). It returns the extent's bounding
// box (a block's extent is its outline, a node's is its box) and the
// first/last boxes.
func (l *blockLayout) layoutColumn(children []graph.Child, cx, top float64, connect bool) (bb *bbox, first, last *blockBox) {
	bb = &bbox{}
	i := 0
	for i < len(children) {
		var itemBB *bbox
		var itemFirst, itemLast *blockBox
		if isAsyncNode(children[i]) {
			j := i
			for j < len(children) && isAsyncNode(children[j]) {
				j++
			}
			itemBB, itemFirst, itemLast = l.placeAsyncRun(children[i:j], cx, top, connect)
			i = j
		} else {
			itemBB, itemFirst, itemLast = l.placeChild(children[i], cx, top, connect)
			i++
		}
		bb.fold(itemBB)
		if first == nil {
			first = itemFirst
		}
		last = itemLast
		top = itemBB.maxY + graphRowGap
	}
	return bb, first, last
}

// isAsyncNode reports whether the child is an async node (a Go construct).
func isAsyncNode(c graph.Child) bool {
	return c.Node != nil && c.Node.Kind == graph.NodeAsync
}

// placeChild places one child (a node, an if fork, or a block) and returns
// its bounding box and first/last boxes.
func (l *blockLayout) placeChild(c graph.Child, cx, top float64, connect bool) (bb *bbox, first, last *blockBox) {
	if c.Node != nil {
		return l.placeNodeChild(c.Node, cx, top, connect)
	}
	if c.Block.Type == graph.BlockIf {
		return l.placeIfChild(c.Block, cx, top, connect)
	}
	return l.placeBlockChild(c.Block, cx, top, connect)
}

// placeNodeChild places a single node box and connects it to the main flow.
func (l *blockLayout) placeNodeChild(n *graph.Node, cx, top float64, connect bool) (bb *bbox, first, last *blockBox) {
	box := l.placeNode(n, cx, top, nodeBoxW(n.Name, n.Kind))
	if connect {
		l.connectTo(box, box)
	}
	bb = &bbox{}
	bb.add(box.x, box.y, box.x+box.w, box.y+box.h)
	return bb, box, box
}

// placeIfChild places the if's branches (side by side) and wires the fork's
// incoming/outgoing arrows.
func (l *blockLayout) placeIfChild(b *graph.Block, cx, top float64, connect bool) (bb *bbox, first, last *blockBox) {
	branchBB, branchFirsts, branchLasts := l.layoutIf(b, cx, top)
	if connect {
		if l.prev != nil {
			l.forkIn(l.prev, branchFirsts)
		}
		l.forkPending = append(l.forkPending, branchLasts...)
		l.prev = nil
	}
	return branchBB, firstOf(branchFirsts), lastOf(branchLasts)
}

// placeBlockChild places a sequence/loop/join block (recurses) and draws
// its outline. It returns the outline's bounding box — the block's
// visible extent, so the row gap to the next item is measured from the
// outline, not from the content.
func (l *blockLayout) placeBlockChild(b *graph.Block, cx, top float64, connect bool) (bb *bbox, first, last *blockBox) {
	contentBB, subFirst, subLast := l.layoutColumn(b.Children, cx, top, connect)
	o := outlineOf(contentBB, b.Level, b.Type == graph.BlockLoop)
	l.outlines = append(l.outlines, o)
	bb = &bbox{}
	bb.add(o.x, o.y, o.x+o.w, o.y+o.h)
	return bb, subFirst, subLast
}

func firstOf(boxes []*blockBox) *blockBox {
	if len(boxes) > 0 {
		return boxes[0]
	}
	return nil
}

func lastOf(boxes []*blockBox) *blockBox {
	if len(boxes) > 0 {
		return boxes[len(boxes)-1]
	}
	return nil
}

// connectTo draws the arrow into first (from each forkPending box, or from
// prev) and sets prev to last.
func (l *blockLayout) connectTo(first, last *blockBox) {
	if len(l.forkPending) > 0 {
		for _, p := range l.forkPending {
			l.edges = append(l.edges, blockEdge{p.x + p.w/2, p.y + p.h, first.x + first.w/2, first.y})
		}
		l.forkPending = nil
	} else if l.prev != nil {
		l.edges = append(l.edges, blockEdge{l.prev.x + l.prev.w/2, l.prev.y + l.prev.h, first.x + first.w/2, first.y})
	}
	l.prev = last
}

// placeNode places a box centered at (cx, top) and records it.
func (l *blockLayout) placeNode(n *graph.Node, cx, top, w float64) *blockBox {
	box := blockBox{
		x: cx - w/2, y: top, w: w, h: graphNodeH,
		text: n.Name, kind: n.Kind,
	}
	l.boxes = append(l.boxes, box)
	return &l.boxes[len(l.boxes)-1]
}

// placeAsyncRun places the async run (side by side, the ∥ mark between the
// boxes) centered at cx, starting at top. The run is one logical unit for
// the arrows: the incoming/outgoing edges use the group's center, not one
// of the boxes.
func (l *blockLayout) placeAsyncRun(run []graph.Child, cx, top float64, connect bool) (bb *bbox, first, last *blockBox) {
	bb = &bbox{}
	totalW := 0.0
	for _, c := range run {
		totalW += nodeBoxW(c.Node.Name, c.Node.Kind)
	}
	totalW += graphColGap * float64(len(run)-1)
	x := cx - totalW/2
	for i, c := range run {
		w := nodeBoxW(c.Node.Name, c.Node.Kind)
		box := l.placeNode(c.Node, x+w/2, top, w)
		if i == 0 {
			first = box
		}
		last = box
		bb.add(box.x, box.y, box.x+box.w, box.y+box.h)
		if i > 0 {
			l.asyncMarks = append(l.asyncMarks, point{box.x - graphColGap/2, top + graphNodeH/2})
		}
		x += w + graphColGap
	}
	if connect {
		group := &blockBox{x: bb.minX, y: top, w: bb.maxX - bb.minX, h: graphNodeH}
		l.connectTo(group, group)
		first, last = group, group
	}
	return bb, first, last
}

// canvas sizes the canvas to contain everything: the boxes and the block
// outlines (the L0 outline is the outermost), with room for the outline
// stroke.
func (l *blockLayout) canvas() {
	all := bbox{}
	for _, b := range l.boxes {
		all.add(b.x, b.y, b.x+b.w, b.y+b.h)
	}
	for _, o := range l.outlines {
		all.add(o.x, o.y, o.x+o.w, o.y+o.h)
	}
	l.width = (all.maxX - all.minX) + 2*graphCanvasPad
	l.height = (all.maxY - all.minY) + 2*graphCanvasPad
	shiftX := all.minX - graphCanvasPad
	shiftY := all.minY - graphCanvasPad
	for i := range l.boxes {
		l.boxes[i].x -= shiftX
		l.boxes[i].y -= shiftY
	}
	for i := range l.outlines {
		l.outlines[i].x -= shiftX
		l.outlines[i].y -= shiftY
	}
	for i := range l.edges {
		l.edges[i].x1 -= shiftX
		l.edges[i].y1 -= shiftY
		l.edges[i].x2 -= shiftX
		l.edges[i].y2 -= shiftY
	}
	for i := range l.asyncMarks {
		l.asyncMarks[i].x -= shiftX
		l.asyncMarks[i].y -= shiftY
	}
}
