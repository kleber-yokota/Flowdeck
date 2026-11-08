// block_layout.go is the recursive block-DAG layout: the main-flow cursor
// (the vertical stack and the per-child dispatch) and the if-fork (the
// branches side by side, equal width, the skip branch forks as arrows only,
// the width metrics, and the fork wiring).
package block

import (
	"flowdeck/core/graph"
	graphshared "flowdeck/web/graph"
)

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
		top = itemBB.maxY + graphshared.GraphRowGap
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
	box := l.placeNode(n, cx, top, graphshared.NodeBoxW(n.Name, n.Kind))
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
		x += maxW + graphshared.GraphColGap
	}
	return bb, firsts, lasts
}

// forkWidth is the total width of a fork of n equal-width branches.
func forkWidth(n int, maxW float64) float64 {
	return float64(n)*maxW + graphshared.GraphColGap*float64(n-1)
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
			cw = graphshared.NodeBoxW(c.Node.Name, c.Node.Kind)
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
		return forkWidth(len(ifBranches(c)), forkMaxWidth(ifBranches(c))) + 2*graphshared.GraphPadX
	}
	return measureContentWidth(c.Children) + 2*graphshared.GraphPadX
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
