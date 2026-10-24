package web

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"
)

// pathNode is a step node of the paths graph: the continuations are
// Children — one child per distinct next step, so the if forks the
// children. Terminal marks a step where some path ends.
type pathNode struct {
	Step     string
	Terminal bool
	Children []pathNode
}

// pathsGraph builds the tree of a workflow's possible paths: the common
// prefix is drawn once and the if forks the branches (not one row per
// path).
func pathsGraph(paths [][]string) []pathNode {
	groups := map[string][][]string{}
	order := []string{}
	for _, p := range paths {
		if len(p) == 0 {
			continue
		}
		f := p[0]
		if _, ok := groups[f]; !ok {
			order = append(order, f)
		}
		groups[f] = append(groups[f], p)
	}
	roots := make([]pathNode, 0, len(order))
	for _, f := range order {
		roots = append(roots, nodeFromPaths(groups[f]))
	}
	return roots
}

func nodeFromPaths(paths [][]string) pathNode {
	node := pathNode{Step: paths[0][0]}
	rest := make([][]string, 0, len(paths))
	for _, p := range paths {
		if len(p) == 1 {
			node.Terminal = true
		} else {
			rest = append(rest, p[1:])
		}
	}
	groups := map[string][][]string{}
	order := []string{}
	for _, r := range rest {
		f := r[0]
		if _, ok := groups[f]; !ok {
			order = append(order, f)
		}
		groups[f] = append(groups[f], r)
	}
	node.Children = make([]pathNode, 0, len(order))
	for _, f := range order {
		node.Children = append(node.Children, nodeFromPaths(groups[f]))
	}
	return node
}

// pathsToSVG renders the paths graph (phase 4, STEP 2) as a static SVG:
// the steps are nodes, each continuation is an arrow, and the if forks the
// branches — the common prefix is drawn once.
func pathsToSVG(paths [][]string) template.HTML {
	g := newGraphLayout(pathsGraph(paths))
	return template.HTML(g.svg())
}

// graphLayout is the measured paths graph: one rect per step and one
// arrow per continuation. The leaves stack rows and each node centers on
// its children — a linear chain runs in one row and the if forks the rows.
type graphLayout struct {
	maxDepth int
	nodes    []graphNode
	edges    [][2]graphNode
	width    float64
	height   float64
}

type graphNode struct {
	step     string
	x, y, w  float64 // x/y are the node's center
	children []graphNode
}

// The graph metrics (px): 14px monospace text, ~0.6x the font per char.
const (
	graphCharW  = 8.4
	graphPadX   = 14.0
	graphNodeH  = 36.0
	graphColGap = 44.0
	graphRowGap = 24.0
	graphMargin = 12.0
)

func newGraphLayout(roots []pathNode) *graphLayout {
	g := &graphLayout{}
	var maxW float64
	leaves := 0
	var measure func(n pathNode, depth int)
	measure = func(n pathNode, depth int) {
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
			measure(c, depth+1)
		}
	}
	for _, r := range roots {
		measure(r, 0)
	}
	if leaves == 0 {
		leaves = 1 // no paths: a valid (empty) canvas
	}
	colW := maxW + graphColGap
	rowH := graphNodeH + graphRowGap
	g.width = 2*graphMargin + float64(g.maxDepth+1)*colW
	g.height = 2*graphMargin + (float64(leaves)-0.5)*rowH + graphNodeH/2

	slot := 0.0
	placed := make([]graphNode, 0, len(roots))
	var place func(n pathNode, depth int) graphNode
	place = func(n pathNode, depth int) graphNode {
		node := graphNode{
			step: n.Step,
			w:    graphCharW*float64(len(n.Step)) + 2*graphPadX,
			x:    graphMargin + float64(depth)*colW,
		}
		node.x += node.w / 2
		if len(n.Children) == 0 {
			node.y = graphMargin + (slot+0.5)*rowH
			slot++
		} else {
			sum := 0.0
			for _, c := range n.Children {
				node.children = append(node.children, place(c, depth+1))
			}
			for _, c := range node.children {
				sum += c.y
			}
			node.y = sum / float64(len(node.children))
		}
		return node
	}
	for _, r := range roots {
		placed = append(placed, place(r, 0))
	}
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
	return g
}

// svg emits the static SVG: edges under the nodes, one rect + label per
// step. The colors come from the stylesheet (g-node/g-edge/g-text/g-arrow).
func (g *graphLayout) svg() string {
	w, h := int(math.Round(g.width)), int(math.Round(g.height))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="paths-graph" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="possible paths">`, w, h, w, h)
	b.WriteString(`<defs><marker id="flowdeck-arrow" markerWidth="10" markerHeight="10" refX="9" refY="5" orient="auto" markerUnits="userSpaceOnUse"><path class="g-arrow" d="M0,0 L10,5 L0,10 Z"/></marker></defs>`)
	for _, e := range g.edges {
		fmt.Fprintf(&b, `<line class="g-edge" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#flowdeck-arrow)"/>`,
			num(e[0].x+e[0].w/2), num(e[0].y), num(e[1].x-e[1].w/2-1), num(e[1].y))
	}
	for _, n := range g.nodes {
		fmt.Fprintf(&b, `<rect class="g-node" x="%s" y="%s" width="%s" height="%s" rx="5"/>`,
			num(n.x-n.w/2), num(n.y-graphNodeH/2), num(n.w), num(graphNodeH))
		fmt.Fprintf(&b, `<text class="g-text" x="%s" y="%s" text-anchor="middle">%s</text>`,
			num(n.x), num(n.y+5), template.HTMLEscapeString(n.step))
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// num formats a layout coordinate (one decimal is enough for px).
func num(f float64) string {
	return strconv.FormatFloat(f, 'f', 1, 64)
}
