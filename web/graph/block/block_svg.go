// block_svg.go is the block-DAG entry and SVG emission: BlockToSVG, the
// boxes with their kind symbols, the edges, and the loop symbols.
package block

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"

	"flowdeck/core/graph"
	graphshared "flowdeck/web/graph"
)

// BlockToSVG renders the block tree (the possible-paths model) as the
// block-DAG SVG. A nil tree (or an empty root) is the empty SVG.
func BlockToSVG(wg *graph.WorkflowGraph) template.HTML {
	if wg == nil || wg.Root == nil {
		return template.HTML("")
	}
	l := &blockLayout{}
	rootBB, rootFirst, rootLast := l.layoutColumn(wg.Root.Children, 0, graphshared.GraphMargin, true)
	_ = rootFirst
	_ = rootLast
	// The root outline (the L0 sequence), a bit larger than the inner
	// outlines so it never shares an edge with a block that fills the
	// canvas (a trailing loop).
	l.outlines = append(l.outlines, outlinePadded(rootBB, 0, false, graphshared.GraphRootPad))
	l.canvas()
	return template.HTML(l.svg())
}

// svg emits the block-DAG SVG: the outlines under the boxes, the boxes with
// the kind's symbol, the edges (arrows), and the loop symbols.
func (l *blockLayout) svg() string {
	w, h := int(math.Round(l.width*graphshared.GraphScale)), int(math.Round(l.height*graphshared.GraphScale))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="block-graph" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="block graph"><g transform="scale(%s)">`,
		w, h, w, h, strconv.FormatFloat(graphshared.GraphScale, 'f', -1, 64))
	b.WriteString(graphshared.ArrowDefs)
	for _, o := range l.outlines {
		fmt.Fprintf(&b, `<rect class="g-block %s" x="%s" y="%s" width="%s" height="%s" rx="8"/>`,
			levelClass(o.level), graphshared.Num(o.x), graphshared.Num(o.y), graphshared.Num(o.w), graphshared.Num(o.h))
		if o.isLoop {
			// The loop symbol in the outline's top-right corner (the 14px
			// padding holds it): an open 300° arc with an arrowhead.
			fmt.Fprintf(&b, `<g class="loop-symbol" transform="translate(%s,%s) scale(0.875)"><path d="M5.5,0 A5.5,5.5 0 1 1 2.75,-4.76" fill="none" stroke-width="2"/><polygon points="6.65,-2.51 4.25,-7.36 1.25,-2.16"/></g>`,
				graphshared.Num(o.x+o.w-7), graphshared.Num(o.y+7))
		}
	}
	for _, e := range l.edges {
		fmt.Fprintf(&b, `<line class="g-edge" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#flowdeck-arrow)"/>`,
			graphshared.Num(e.x1), graphshared.Num(e.y1), graphshared.Num(e.x2), graphshared.Num(e.y2))
	}
	for _, n := range l.boxes {
		fmt.Fprintf(&b, `<rect class="g-node" x="%s" y="%s" width="%s" height="%s" rx="5"/>`,
			graphshared.Num(n.x), graphshared.Num(n.y), graphshared.Num(n.w), graphshared.Num(n.h))
		labelX := n.x + n.w/2
		if n.kind == graph.NodeChild {
			// The child symbol (◉: ring + dot) in the box's right zone.
			sx := n.x + n.w - graphshared.GraphPadX - graphshared.GraphSymW/2
			labelX = n.x + (n.w-graphshared.GraphSymW)/2
			fmt.Fprintf(&b, `<g class="child-symbol" transform="translate(%s,%s)"><circle class="child-ring" r="5"/><circle class="child-dot" r="2"/></g>`,
				graphshared.Num(sx), graphshared.Num(n.y+n.h/2))
		}
		if n.kind == graph.NodeWait {
			// The wait symbol (…: three dots) in the box's right zone.
			sx := n.x + n.w - graphshared.GraphPadX - graphshared.GraphSymW/2
			labelX = n.x + (n.w-graphshared.GraphSymW)/2
			ny := n.y + n.h/2
			fmt.Fprintf(&b, `<g class="wait-symbol"><circle cx="%s" cy="%s" r="1.5"/><circle cx="%s" cy="%s" r="1.5"/><circle cx="%s" cy="%s" r="1.5"/></g>`,
				graphshared.Num(sx-6), graphshared.Num(ny), graphshared.Num(sx), graphshared.Num(ny), graphshared.Num(sx+6), graphshared.Num(ny))
		}
		fmt.Fprintf(&b, `<text class="g-text" x="%s" y="%s" text-anchor="middle">%s</text>`,
			graphshared.Num(labelX), graphshared.Num(n.y+n.h/2+5), template.HTMLEscapeString(n.text))
	}
	for _, m := range l.asyncMarks {
		// The ∥ mark between the boxes of an async run.
		fmt.Fprintf(&b, `<g class="async-symbol" transform="translate(%s,%s)"><path d="M-3,-6 L-3,6 M3,-6 L3,6"/></g>`,
			graphshared.Num(m.x), graphshared.Num(m.y))
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
