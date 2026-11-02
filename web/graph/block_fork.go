// block_fork.go lays out the if fork: the branches side by side (equal
// width, the skip branch forks as arrows only), the width metrics the fork
// width uses, and the fork wiring (the incoming arrows).
package graph

import (
	"flowdeck/core/graph"
)

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

// forkIn draws the incoming fork arrow (from the incoming box to each
// branch's first box).
func (l *blockLayout) forkIn(incoming *blockBox, firsts []*blockBox) {
	for _, f := range firsts {
		if f != nil {
			l.edges = append(l.edges, blockEdge{incoming.x + incoming.w/2, incoming.y + incoming.h, f.x + f.w/2, f.y})
		}
	}
}
