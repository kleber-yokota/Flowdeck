package executed

import (
	coregraph "flowdeck/core/graph"
	graphshared "flowdeck/web/graph"
)

// graphLayout is the measured paths graph: one rect per step and one
// arrow per continuation. Transposed (task 02): the depth runs top to
// bottom (y), the leaves take columns (x) and each node centers on its
// children's x — a linear chain runs in one column and the fork spreads
// its children across the columns below it.
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
	kind     coregraph.NodeKind // the glyph kind (step · child · wait)
	loop     bool               // the loop group: the loop arc
	title    string             // the collapse group: the full member list (<title>)
}

func newGraphLayout(roots []pathNode) *graphLayout {
	g := &graphLayout{}
	colW, rowH, maxW := g.measure(roots)
	g.collect(g.place(roots, colW, rowH, maxW))
	return g
}

// isTerminal reports the invisible end-of-chain marker (no box, no
// children — the layout skips it).
func isTerminal(n pathNode) bool {
	return n.Step == "" && len(n.Children) == 0
}

// visibleKids counts the children that have a box (the terminal marker
// aside).
func visibleKids(n pathNode) int {
	kids := 0
	for _, c := range n.Children {
		if !isTerminal(c) {
			kids++
		}
	}
	return kids
}

// depthStats is the single walk of measure: the max depth, the widest
// box, and the leaf count (the invisible group nodes consume no depth of
// their own).
type depthStats struct {
	maxDepth int
	maxW     float64
	leaves   int
}

// walkDepth measures one subtree: the depth, the widest box, and the leaf
// count. The invisible group node relays its depth to the members (they
// don't consume a depth of their own); the terminal marker is skipped.
func walkDepth(n pathNode, depth int, s *depthStats) {
	if isTerminal(n) {
		return
	}
	if depth > s.maxDepth {
		s.maxDepth = depth
	}
	if n.Step == "" {
		for _, c := range n.Children {
			walkDepth(c, depth, s)
		}
		return
	}
	if w := graphshared.NodeBoxW(n.Step, n.Kind); w > s.maxW {
		s.maxW = w
	}
	if visibleKids(n) == 0 {
		s.leaves++
	}
	for _, c := range n.Children {
		walkDepth(c, depth+1, s)
	}
}

// measure sizes the canvas: colW is the horizontal pitch of a leaf column
// (the widest box + the column gap), rowH the vertical pitch of a depth
// row (the box height + the row gap); the canvas is the tight bounding
// box of the columns and the depth rows (plus the margins).
func (g *graphLayout) measure(roots []pathNode) (colW, rowH, maxW float64) {
	var s depthStats
	for _, r := range roots {
		walkDepth(r, 0, &s)
	}
	if s.leaves == 0 {
		s.leaves = 1 // no path: a valid (empty) canvas
	}
	g.maxDepth = s.maxDepth
	colW = s.maxW + graphshared.GraphColGap
	rowH = graphshared.GraphNodeH + graphshared.GraphRowGap
	g.width = 2*graphshared.GraphMargin + (float64(s.leaves)-1)*colW + s.maxW
	g.height = 2*graphshared.GraphMargin + float64(g.maxDepth)*rowH + graphshared.GraphNodeH
	return colW, rowH, s.maxW
}

// placeState carries the layout parameters and the running leaf slot
// across the recursive placement.
type placeState struct {
	colW, rowH, maxW float64
	slot             float64
}

// place lays out the nodes: the depth runs top to bottom (y), the leaves
// take columns (the slot x) and each parent centers on the average x of
// its visible children. The invisible group node consumes no depth: its
// members take the group's depth row, and the group centers on their x.
func (g *graphLayout) place(roots []pathNode, colW, rowH, maxW float64) []graphNode {
	s := placeState{colW: colW, rowH: rowH, maxW: maxW}
	placed := make([]graphNode, 0, len(roots))
	for _, r := range roots {
		placed = append(placed, s.placeNode(r, 0))
	}
	return placed
}

// placeNode lays out one node (and recurses into its children) — the
// preorder walk takes the slots as the leaves are met, so the slot order
// is the left-to-right reading order.
func (s *placeState) placeNode(n pathNode, depth int) graphNode {
	node := graphNode{
		step:  n.Step,
		state: n.State,
		w:     graphshared.NodeBoxW(n.Step, n.Kind),
		kind:  n.Kind,
		loop:  n.Loop,
		title: n.Title,
	}
	node.y = graphshared.GraphMargin + float64(depth)*s.rowH + graphshared.GraphNodeH/2
	if n.Step == "" {
		return s.placeGroup(n, depth)
	}
	if visibleKids(n) == 0 {
		node.x = graphshared.GraphMargin + s.slot*s.colW + s.maxW/2
		s.slot++
		return node
	}
	s.placeParent(n, depth, &node)
	return node
}

// placeGroup lays out the invisible group node: no box of its own; the
// members take the group's depth row, and the group centers on their x.
func (s *placeState) placeGroup(n pathNode, depth int) graphNode {
	node := graphNode{}
	node.y = graphshared.GraphMargin + float64(depth)*s.rowH + graphshared.GraphNodeH/2
	sum := 0.0
	for _, c := range n.Children {
		if isTerminal(c) {
			continue
		}
		child := s.placeNode(c, depth)
		node.children = append(node.children, child)
		sum += child.x
	}
	if len(node.children) > 0 {
		node.x = sum / float64(len(node.children))
	}
	return node
}

// placeParent lays out a fork: the children take the depth below (y), and
// the parent centers on the average x of its visible children (the
// terminal marker aside).
func (s *placeState) placeParent(n pathNode, depth int, node *graphNode) {
	sum := 0.0
	kids := 0
	for _, c := range n.Children {
		child := s.placeNode(c, depth+1)
		node.children = append(node.children, child)
		if !isTerminal(c) {
			sum += child.x
			kids++
		}
	}
	node.x = sum / float64(kids)
}

// collect walks the placed tree (preorder) into the flat nodes (the order
// the SVG emits) and the continuation edges — the invisible group nodes
// relay their edges: the visible parent reaches each member directly.
func (g *graphLayout) collect(placed []graphNode) {
	for _, r := range placed {
		g.appendNode(r)
		g.appendEdges(r)
	}
}

// appendNode appends the node and its subtree (preorder).
func (g *graphLayout) appendNode(n graphNode) {
	g.nodes = append(g.nodes, n)
	for _, c := range n.children {
		g.appendNode(c)
	}
}

// appendEdges appends the continuation edges of the subtree: a visible
// child is reached directly; the invisible group node relays — the
// visible parent reaches each member directly.
func (g *graphLayout) appendEdges(n graphNode) {
	for _, c := range n.children {
		if c.step == "" {
			g.appendGroupEdges(n, c)
		} else {
			g.appendDirectEdge(n, c)
		}
	}
}

// appendDirectEdge is a visible child: the parent reaches the child
// directly, and the edge continues into the child's subtree.
func (g *graphLayout) appendDirectEdge(n, c graphNode) {
	if n.step != "" {
		g.edges = append(g.edges, [2]graphNode{n, c})
	}
	g.appendEdges(c)
}

// appendGroupEdges is the invisible group node (with children — the
// terminal marker has none and ends nothing): the visible parent reaches
// each member directly, and the edge continues into each member's
// subtree.
func (g *graphLayout) appendGroupEdges(n, c graphNode) {
	for _, m := range c.children {
		if n.step != "" {
			g.edges = append(g.edges, [2]graphNode{n, m})
		}
		g.appendEdges(m)
	}
}
