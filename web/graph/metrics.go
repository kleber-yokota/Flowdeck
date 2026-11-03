// metrics.go is the shared geometry of the two graphs (the executed path
// and the block DAG): the char width, the paddings and gaps, the box
// widths.
package graph

import (
	"flowdeck/core/graph"
)

// The graph metrics (px): 16px monospace text, ~0.69x the font per char.
const (
	graphCharW     = 11.0
	graphPadX      = 14.0 // the inner block outline padding
	graphRootPad   = 20.0 // the root (L0) outline padding: it clears the inner outlines
	graphNodeH     = 48.0 // the readability height: every box is this tall
	graphColGap    = 44.0 // the horizontal gap between columns (the transpose: the leaves' columns)
	graphRowGap    = 24.0 // the vertical gap between depth rows
	graphMargin    = 12.0
	graphCanvasPad = 2.0  // room for the outline stroke beyond the L0 outline
	graphSymW      = 20.0 // the child/wait symbol zone on the box's right
)

// baseBoxW is a step box's width (the text plus the padding).
func baseBoxW(name string) float64 {
	return graphCharW*float64(len(name)) + 2*graphPadX
}

// nodeBoxW is a node box's width (16px monospace text, ~0.69x the font per
// char, plus the padding; the child/wait kinds add the symbol zone).
func nodeBoxW(name string, kind graph.NodeKind) float64 {
	w := baseBoxW(name)
	if kind == graph.NodeChild || kind == graph.NodeWait {
		w += graphSymW
	}
	return w
}
