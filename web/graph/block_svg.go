// block_svg.go emits the block-DAG SVG: the outline geometry (the L0 root
// outline clears the inner blocks), the boxes with their kind symbols, the
// edges, and the 1.25x scale.
package graph

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"

	"flowdeck/core/graph"
)

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
