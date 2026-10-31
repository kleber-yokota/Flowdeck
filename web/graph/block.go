package graph

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"

	"flowdeck/core/graph"
)

// The block-DAG treatment (diagrams/graph/melhoria/webgraph-block-*.html):
// the block tree is received as-is, each block is stroked at its level (the
// level ramp), the node kind is represented (child/wait symbols, async side
// by side, loop symbol), and the result is the SVG.
//
// The layout is top → bottom: the blocks are outlines, the nodes are boxes,
// the if forks the branches side by side, the async is side by side, and the
// loop carries the loop symbol.

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

func outlineOf(bb *bbox, level int, isLoop bool) blockOutline {
	return outlinePadded(bb, level, isLoop, graphPadX)
}

func outlinePadded(bb *bbox, level int, isLoop bool, pad float64) blockOutline {
	return blockOutline{
		x: bb.minX - pad, y: bb.minY - pad,
		w: (bb.maxX - bb.minX) + 2*pad, h: (bb.maxY - bb.minY) + 2*pad,
		level: level, isLoop: isLoop,
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

// layoutIf lays out the if's branches side by side (centered at cx),
// starting at top. It returns the branches' bounding box, the branches'
// first boxes, and the branches' last boxes (the fork's outgoing).
func (l *blockLayout) layoutIf(b *graph.Block, cx, top float64) (bb *bbox, firsts []*blockBox, lasts []*blockBox) {
	bb = &bbox{}
	branches := ifBranches(b)
	// Equal-width branches: the fork is symmetric, so its continuation
	// centers under the middle branch.
	maxW := forkMaxWidth(branches)
	totalW := forkWidth(len(branches), maxW)
	x := cx - totalW/2
	for _, br := range branches {
		branchBB, branchFirst, branchLast := l.layoutColumn(br, x+maxW/2, top, false)
		bb.fold(branchBB)
		if branchFirst != nil {
			firsts = append(firsts, branchFirst)
		}
		if branchLast != nil {
			lasts = append(lasts, branchLast)
		}
		x += maxW + graphColGap
	}
	return bb, firsts, lasts
}

// forkWidth is the total width of a fork of n equal-width branches.
func forkWidth(n int, maxW float64) float64 {
	return float64(n)*maxW + graphColGap*float64(n-1)
}

// forkMaxWidth is the branches' equal width (the max content width).
func forkMaxWidth(branches [][]graph.Child) float64 {
	maxW := 0.0
	for _, br := range branches {
		if w := measureContentWidth(br); w > maxW {
			maxW = w
		}
	}
	return maxW
}

// ifBranches returns the if's branches, in order: the if-body, each else-if
// body, the else body, and the skip (when there is no else).
func ifBranches(b *graph.Block) [][]graph.Child {
	branches := [][]graph.Child{b.Children}
	e := b.Else
	for e != nil {
		if e.Type == graph.BlockIf {
			branches = append(branches, e.Children)
			e = e.Else
		} else {
			branches = append(branches, e.Children)
			e = nil
		}
	}
	if b.Else == nil {
		branches = append(branches, nil) // the skip
	}
	return branches
}

// measureContentWidth is the width of a content (a vertical stack of
// children): the max child width (an if is its branches side by side).
func measureContentWidth(children []graph.Child) float64 {
	var maxW float64
	for _, c := range children {
		var cw float64
		if c.Node != nil {
			cw = nodeBoxW(c.Node.Name, c.Node.Kind)
		} else {
			cw = measureChildWidth(c.Block)
		}
		if cw > maxW {
			maxW = cw
		}
	}
	return maxW
}

func measureChildWidth(c *graph.Block) float64 {
	if c.Type == graph.BlockIf {
		return forkWidth(len(ifBranches(c)), forkMaxWidth(ifBranches(c))) + 2*graphPadX
	}
	return measureContentWidth(c.Children) + 2*graphPadX
}

// nodeBoxW is a node box's width (14px monospace text, ~0.6x the font per
// char, plus the padding; the child/wait kinds add the symbol zone).
func nodeBoxW(name string, kind graph.NodeKind) float64 {
	w := graphCharW*float64(len(name)) + 2*graphPadX
	if kind == graph.NodeChild || kind == graph.NodeWait {
		w += graphSymW
	}
	return w
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

// forkIn draws the incoming fork arrow (from the incoming box to each
// branch's first box).
func (l *blockLayout) forkIn(incoming *blockBox, firsts []*blockBox) {
	for _, f := range firsts {
		if f != nil {
			l.edges = append(l.edges, blockEdge{incoming.x + incoming.w/2, incoming.y + incoming.h, f.x + f.w/2, f.y})
		}
	}
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

// svg emits the block-DAG SVG: the outlines under the boxes, the boxes with
// the kind's symbol, the edges (arrows), and the loop symbols.
func (l *blockLayout) svg() string {
	w, h := int(math.Round(l.width*graphScale)), int(math.Round(l.height*graphScale))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="block-graph" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="block graph"><g transform="scale(%s)">`,
		w, h, w, h, strconv.FormatFloat(graphScale, 'f', -1, 64))
	b.WriteString(`<defs><marker id="flowdeck-arrow" markerWidth="10" markerHeight="10" refX="9" refY="5" orient="auto" markerUnits="userSpaceOnUse"><path class="g-arrow" d="M0,0 L10,5 L0,10 Z"/></marker></defs>`)
	for _, o := range l.outlines {
		fmt.Fprintf(&b, `<rect class="g-block %s" x="%s" y="%s" width="%s" height="%s" rx="8"/>`,
			levelClass(o.level), num(o.x), num(o.y), num(o.w), num(o.h))
		if o.isLoop {
			// The loop symbol in the outline's top-right corner (the 14px
			// padding holds it): an open 300° arc with an arrowhead.
			fmt.Fprintf(&b, `<g class="loop-symbol" transform="translate(%s,%s) scale(0.875)"><path d="M5.5,0 A5.5,5.5 0 1 1 2.75,-4.76" fill="none" stroke-width="2"/><polygon points="6.65,-2.51 4.25,-7.36 1.25,-2.16"/></g>`,
				num(o.x+o.w-7), num(o.y+7))
		}
	}
	for _, e := range l.edges {
		fmt.Fprintf(&b, `<line class="g-edge" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#flowdeck-arrow)"/>`,
			num(e.x1), num(e.y1), num(e.x2), num(e.y2))
	}
	for _, n := range l.boxes {
		fmt.Fprintf(&b, `<rect class="g-node" x="%s" y="%s" width="%s" height="%s" rx="5"/>`,
			num(n.x), num(n.y), num(n.w), num(n.h))
		labelX := n.x + n.w/2
		if n.kind == graph.NodeChild {
			// The child symbol (◉: ring + dot) in the box's right zone.
			sx := n.x + n.w - graphPadX - graphSymW/2
			labelX = n.x + (n.w-graphSymW)/2
			fmt.Fprintf(&b, `<g class="child-symbol" transform="translate(%s,%s)"><circle class="child-ring" r="5"/><circle class="child-dot" r="2"/></g>`,
				num(sx), num(n.y+n.h/2))
		}
		if n.kind == graph.NodeWait {
			// The wait symbol (…: three dots) in the box's right zone.
			sx := n.x + n.w - graphPadX - graphSymW/2
			labelX = n.x + (n.w-graphSymW)/2
			ny := n.y + n.h/2
			fmt.Fprintf(&b, `<g class="wait-symbol"><circle cx="%s" cy="%s" r="1.5"/><circle cx="%s" cy="%s" r="1.5"/><circle cx="%s" cy="%s" r="1.5"/></g>`,
				num(sx-6), num(ny), num(sx), num(ny), num(sx+6), num(ny))
		}
		fmt.Fprintf(&b, `<text class="g-text" x="%s" y="%s" text-anchor="middle">%s</text>`,
			num(labelX), num(n.y+n.h/2+5), template.HTMLEscapeString(n.text))
	}
	for _, m := range l.asyncMarks {
		// The ∥ mark between the boxes of an async run.
		fmt.Fprintf(&b, `<g class="async-symbol" transform="translate(%s,%s)"><path d="M-3,-6 L-3,6 M3,-6 L3,6"/></g>`,
			num(m.x), num(m.y))
	}
	b.WriteString(`</g></svg>`)
	return b.String()
}

// levelClass is the block outline's level class (the level ramp): L0 · L1 ·
// L2 · L3+ (reusing L2).
func levelClass(level int) string {
	switch {
	case level <= 0:
		return "l0"
	case level == 1:
		return "l1"
	case level == 2:
		return "l2"
	}
	return "l2" // L3+ reuses L2
}
