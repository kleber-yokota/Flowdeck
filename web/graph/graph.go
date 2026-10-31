// Package graph is the SVG layout of the paths graphs: the possible-paths
// map (phase 4, STEP 2) and the executed path of a run (phase 4, STEP 3)
// share the same drawing — one rect per step and one arrow per
// continuation, the if forks the branches.
package graph

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"

	"flowdeck/core"
)

// pathNode is a step node of the paths graph: the continuations are
// Children — one child per distinct next step, so the if forks the
// children. Terminal marks a step where some path ends. State is the
// step's state on the executed path (success/error) — empty on the
// possible-paths map (phase 4, STEP 2), where no step has run.
type pathNode struct {
	Step     string
	Terminal bool
	Children []pathNode
	State    string
}

// graphLayout is the measured paths graph: one rect per step and one
// arrow per continuation. The leaves stack rows and each node centers on
// its children — a linear chain runs in one row and the if forks the rows.
type graphLayout struct {
	maxDepth int
	label    string // aria-label of the svg
	nodes    []graphNode
	edges    [][2]graphNode
	width    float64
	height   float64
}

type graphNode struct {
	step     string
	state    string  // executed-path state (success/error); empty on possible paths
	x, y, w  float64 // x/y are the node's center
	children []graphNode
}

// The graph metrics (px): 14px monospace text, ~0.6x the font per char.
const (
	graphCharW     = 8.4
	graphPadX      = 14.0 // the inner block outline padding
	graphRootPad   = 20.0 // the root (L0) outline padding: it clears the inner outlines
	graphNodeH     = 36.0
	graphColGap    = 44.0
	graphRowGap    = 24.0
	graphMargin    = 12.0
	graphCanvasPad = 2.0  // room for the outline stroke beyond the L0 outline
	graphScale     = 1.25 // the svg is emitted 1.25× the content canvas (readability)
	graphSymW      = 20.0 // the child/wait symbol zone on the box's right
)

func newGraphLayout(roots []pathNode) *graphLayout {
	g := &graphLayout{}
	colW, rowH := g.measure(roots)
	g.collect(g.place(roots, colW, rowH))
	return g
}

// measure sizes the canvas: the max depth is kept on the layout, the
// leaf count (the rows) and the max node width set the width/height, and
// the column/row pitch is returned for place.
func (g *graphLayout) measure(roots []pathNode) (colW, rowH float64) {
	var maxW float64
	leaves := 0
	var m func(n pathNode, depth int)
	m = func(n pathNode, depth int) {
		w := graphCharW*float64(len(n.Step)) + 2*graphPadX
		if w > maxW {
			maxW = w
		}
		if depth > g.maxDepth {
			g.maxDepth = depth
		}
		if len(n.Children) == 0 {
			leaves++
		}
		for _, c := range n.Children {
			m(c, depth+1)
		}
	}
	for _, r := range roots {
		m(r, 0)
	}
	if leaves == 0 {
		leaves = 1 // no paths: a valid (empty) canvas
	}
	colW = maxW + graphColGap
	rowH = graphNodeH + graphRowGap
	g.width = 2*graphMargin + float64(g.maxDepth+1)*colW
	g.height = 2*graphMargin + (float64(leaves)-0.5)*rowH + graphNodeH/2
	return colW, rowH
}

// place lays out the nodes: the leaves stack rows and each parent centers
// on the average y of its children — a linear chain runs in one row and
// the if forks the rows.
func (g *graphLayout) place(roots []pathNode, colW, rowH float64) []graphNode {
	slot := 0.0
	var place func(n pathNode, depth int) graphNode
	place = func(n pathNode, depth int) graphNode {
		node := graphNode{
			step:  n.Step,
			state: n.State,
			w:     graphCharW*float64(len(n.Step)) + 2*graphPadX,
			x:     graphMargin + float64(depth)*colW,
		}
		node.x += node.w / 2
		if len(n.Children) == 0 {
			node.y = graphMargin + (slot+0.5)*rowH
			slot++
		} else {
			sum := 0.0
			for _, c := range n.Children {
				child := place(c, depth+1)
				node.children = append(node.children, child)
				sum += child.y
			}
			node.y = sum / float64(len(node.children))
		}
		return node
	}
	placed := make([]graphNode, 0, len(roots))
	for _, r := range roots {
		placed = append(placed, place(r, 0))
	}
	return placed
}

// collect appends the placed nodes to the layout, one edge per
// continuation (parent -> child).
func (g *graphLayout) collect(placed []graphNode) {
	var walk func(n graphNode)
	walk = func(n graphNode) {
		g.nodes = append(g.nodes, n)
		for _, c := range n.children {
			g.edges = append(g.edges, [2]graphNode{n, c})
			walk(c)
		}
	}
	for _, r := range placed {
		walk(r)
	}
}

// svg emits the static SVG: edges under the nodes, one rect + label per
// step. The colors come from the stylesheet (g-node/g-edge/g-text/g-arrow).
func (g *graphLayout) svg() string {
	w, h := int(math.Round(g.width*graphScale)), int(math.Round(g.height*graphScale))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="paths-graph" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s"><g transform="scale(%s)">`,
		w, h, w, h, g.label, strconv.FormatFloat(graphScale, 'f', -1, 64))
	b.WriteString(`<defs><marker id="flowdeck-arrow" markerWidth="10" markerHeight="10" refX="9" refY="5" orient="auto" markerUnits="userSpaceOnUse"><path class="g-arrow" d="M0,0 L10,5 L0,10 Z"/></marker></defs>`)
	for _, e := range g.edges {
		fmt.Fprintf(&b, `<line class="g-edge" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#flowdeck-arrow)"/>`,
			num(e[0].x+e[0].w/2), num(e[0].y), num(e[1].x-e[1].w/2-1), num(e[1].y))
	}
	for _, n := range g.nodes {
		cls := "g-node"
		if n.state != "" {
			cls += " " + n.state
		}
		fmt.Fprintf(&b, `<rect class="%s" x="%s" y="%s" width="%s" height="%s" rx="5"/>`,
			cls, num(n.x-n.w/2), num(n.y-graphNodeH/2), num(n.w), num(graphNodeH))
		fmt.Fprintf(&b, `<text class="g-text" x="%s" y="%s" text-anchor="middle">%s</text>`,
			num(n.x), num(n.y+5), template.HTMLEscapeString(n.step))
	}
	b.WriteString(`</g></svg>`)
	return b.String()
}

// ExecutedPathToSVG renders the executed path of a run (phase 4, STEP 3)
// as the same graph as the possible paths, with the single row of the
// steps that ran, in execution order: each node is colored by the step's
// state (success/error — every executed step is terminal). It is separate
// from the boot map: the path comes from Postgres, the map from the code.
func ExecutedPathToSVG(steps []core.Step) template.HTML {
	nodes := make([]pathNode, 0, len(steps))
	for _, s := range steps {
		nodes = append(nodes, pathNode{Step: s.Name, State: string(s.State)})
	}
	for i := len(nodes) - 1; i > 0; i-- {
		nodes[i-1].Children = []pathNode{nodes[i]}
	}
	// Only the first step is a root: the rest hang off it as continuations.
	roots := []pathNode{}
	if len(nodes) > 0 {
		roots = []pathNode{nodes[0]}
	}
	g := newGraphLayout(roots)
	g.label = "executed path"
	return template.HTML(g.svg())
}

// num formats a layout coordinate (one decimal is enough for px).
func num(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}
