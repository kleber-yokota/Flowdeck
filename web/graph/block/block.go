// Package graph renders the workflow graphs as SVG: the executed path
// (a row of steps) and the possible paths (the block-DAG from
// core/graph.WorkflowGraph, no execution data).
//
// The block layout (block.go · block_layout.go) measures and places the
// boxes, outlines, and edges; block_svg.go emits the SVG (the entry). The
// shared geometry (metrics.go) and the shared svg emission (svg.go) keep
// the two graphs reading as one family.
package block

import (
	"flowdeck/core/graph"
	graphshared "flowdeck/web/graph"
)

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
		x: cx - w/2, y: top, w: w, h: graphshared.GraphNodeH,
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
		totalW += graphshared.NodeBoxW(c.Node.Name, c.Node.Kind)
	}
	totalW += graphshared.GraphColGap * float64(len(run)-1)
	x := cx - totalW/2
	for i, c := range run {
		w := graphshared.NodeBoxW(c.Node.Name, c.Node.Kind)
		box := l.placeNode(c.Node, x+w/2, top, w)
		if i == 0 {
			first = box
		}
		last = box
		bb.add(box.x, box.y, box.x+box.w, box.y+box.h)
		if i > 0 {
			l.asyncMarks = append(l.asyncMarks, point{box.x - graphshared.GraphColGap/2, top + graphshared.GraphNodeH/2})
		}
		x += w + graphshared.GraphColGap
	}
	if connect {
		group := &blockBox{x: bb.minX, y: top, w: bb.maxX - bb.minX, h: graphshared.GraphNodeH}
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
	l.width = (all.maxX - all.minX) + 2*graphshared.GraphCanvasPad
	l.height = (all.maxY - all.minY) + 2*graphshared.GraphCanvasPad
	shiftX := all.minX - graphshared.GraphCanvasPad
	shiftY := all.minY - graphshared.GraphCanvasPad
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

// outlineOf is the padded outline of a content's bounding box (the
// inner-block padding).
func outlineOf(bb *bbox, level int, isLoop bool) blockOutline {
	return outlinePadded(bb, level, isLoop, graphshared.GraphPadX)
}

// outlinePadded is the outline of a bounding box, padded: the L0 root
// outline (a bit larger) clears the inner blocks.
func outlinePadded(bb *bbox, level int, isLoop bool, pad float64) blockOutline {
	return blockOutline{
		x: bb.minX - pad, y: bb.minY - pad,
		w: (bb.maxX - bb.minX) + 2*pad, h: (bb.maxY - bb.minY) + 2*pad,
		level: level, isLoop: isLoop,
	}
}
