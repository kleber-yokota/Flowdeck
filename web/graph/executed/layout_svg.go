package executed

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"

	coregraph "flowdeck/core/graph"
	graphshared "flowdeck/web/graph"
)

// svg emits the SVG: the arrow defs, the edges, the boxes (with the
// state class · the child/wait symbol · the loop arc · the collapse
// group's <title> · the label), and the ∥ marks between the sibling
// member boxes of the parallel groups.
func (g *graphLayout) svg() string {
	w, h := int(math.Round(g.width*graphshared.GraphScale)), int(math.Round(g.height*graphshared.GraphScale))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="paths-graph" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s"><g transform="scale(%s)">`,
		w, h, w, h, g.label, strconv.FormatFloat(graphshared.GraphScale, 'f', -1, 64))
	b.WriteString(graphshared.ArrowDefs)
	for _, e := range g.edges {
		g.svgEdge(&b, e)
	}
	for _, n := range g.nodes {
		g.svgNode(&b, n)
	}
	g.svgAsyncMarks(&b)
	b.WriteString(`</g></svg>`)
	return b.String()
}

// svgEdge emits one continuation edge (the arrow from the parent's bottom
// to the child's top).
func (g *graphLayout) svgEdge(b *strings.Builder, e [2]graphNode) {
	fmt.Fprintf(b, `<line class="g-edge" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#flowdeck-arrow)"/>`,
		graphshared.Num(e[0].x), graphshared.Num(e[0].y+graphshared.GraphNodeH/2), graphshared.Num(e[1].x), graphshared.Num(e[1].y-graphshared.GraphNodeH/2-1))
}

// svgNode emits one box: the rect (the collapse group carries the member
// list in <title>; the live tail is the dashed tail box — no state
// class), the child/wait symbol, the loop arc, and the label (centered in
// the text zone when the symbol zone is present).
func (g *graphLayout) svgNode(b *strings.Builder, n graphNode) {
	if n.step == "" {
		return // the invisible group and the terminal marker: no box
	}
	cls := "g-node"
	if n.tail {
		cls += " tail"
	} else if n.state != "" {
		cls += " " + n.state
	}
	if n.title != "" {
		fmt.Fprintf(b, `<rect class="%s" x="%s" y="%s" width="%s" height="%s" rx="5"><title>%s</title></rect>`,
			cls, graphshared.Num(n.x-n.w/2), graphshared.Num(n.y-graphshared.GraphNodeH/2), graphshared.Num(n.w), graphshared.Num(graphshared.GraphNodeH),
			template.HTMLEscapeString(n.title))
	} else {
		fmt.Fprintf(b, `<rect class="%s" x="%s" y="%s" width="%s" height="%s" rx="5"/>`,
			cls, graphshared.Num(n.x-n.w/2), graphshared.Num(n.y-graphshared.GraphNodeH/2), graphshared.Num(n.w), graphshared.Num(graphshared.GraphNodeH))
	}
	svgSymbol(b, n)
	if n.loop {
		fmt.Fprintf(b, `<g class="loop-symbol" transform="translate(%s,%s) scale(0.875)"><path d="M5.5,0 A5.5,5.5 0 1 1 2.75,-4.76" fill="none" stroke-width="2"/><polygon points="6.65,-2.51 4.25,-7.36 1.25,-2.16"/></g>`,
			graphshared.Num(n.x+n.w/2-7), graphshared.Num(n.y-graphshared.GraphNodeH/2+7))
	}
	labelX := n.x
	if n.kind == coregraph.NodeChild || n.kind == coregraph.NodeWait {
		labelX = n.x - graphshared.GraphSymW/2 // center the label in the text zone
	}
	textCls := "g-text"
	if n.tail {
		textCls += " tail"
	}
	fmt.Fprintf(b, `<text class="%s" x="%s" y="%s" text-anchor="middle">%s</text>`,
		textCls, graphshared.Num(labelX), graphshared.Num(n.y+5), template.HTMLEscapeString(n.step))
}

// svgSymbol emits the box's glyph: the child symbol (◉: ring + dot) or the
// wait symbol (…: three dots) in the box's right zone.
func svgSymbol(b *strings.Builder, n graphNode) {
	sx := n.x + n.w/2 - graphshared.GraphPadX - graphshared.GraphSymW/2
	switch n.kind {
	case coregraph.NodeChild:
		fmt.Fprintf(b, `<g class="child-symbol" transform="translate(%s,%s)"><circle class="child-ring" r="5"/><circle class="child-dot" r="2"/></g>`,
			graphshared.Num(sx), graphshared.Num(n.y))
	case coregraph.NodeWait:
		fmt.Fprintf(b, `<g class="wait-symbol"><circle cx="%s" cy="%s" r="1.5"/><circle cx="%s" cy="%s" r="1.5"/><circle cx="%s" cy="%s" r="1.5"/></g>`,
			graphshared.Num(sx-6), graphshared.Num(n.y), graphshared.Num(sx), graphshared.Num(n.y), graphshared.Num(sx+6), graphshared.Num(n.y))
	}
}

// svgAsyncMarks emits the ∥ mark (async-symbol) between the sibling member
// boxes of every parallel group.
func (g *graphLayout) svgAsyncMarks(b *strings.Builder) {
	for _, n := range g.nodes {
		if n.step == "" && len(n.children) >= 2 {
			g.svgGroupMarks(b, n)
		}
	}
}

// svgGroupMarks emits one ∥ mark per member pair of the group.
func (g *graphLayout) svgGroupMarks(b *strings.Builder, n graphNode) {
	for i := 1; i < len(n.children); i++ {
		a, c := n.children[i-1], n.children[i]
		mx := (a.x + a.w/2 + c.x - c.w/2) / 2
		fmt.Fprintf(b, `<g class="async-symbol" transform="translate(%s,%s)"><path d="M-3,-6 L-3,6 M3,-6 L3,6"/></g>`,
			graphshared.Num(mx), graphshared.Num(a.y))
	}
}
