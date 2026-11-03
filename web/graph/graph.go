package graph

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
	"strings"

	coregraph "flowdeck/core/graph"
	"flowdeck/core/reads"
)

// pathNode is a step node of the paths graph: the continuations are
// Children — one child per distinct next step, so the if forks the
// children. Terminal marks a step where some path ends. State is the
// step's state on the executed path (success/error) — empty on the
// possible-paths map (phase 4, STEP 2), where no step has run.
//
// The grouping pass (task 02) annotates the nodes: Kind is the glyph
// (step · child · wait), Count is the ×N of the loop/collapse group, Loop
// the loop arc, Title the collapse group's full member list (the box's
// <title>). The invisible nodes (Step "") are the group nodes (their
// children are the parallel group's member boxes) and the terminal marker
// (no children) at the end of the chain.
type pathNode struct {
	Step     string
	Terminal bool
	Children []pathNode
	State    string
	Kind     coregraph.NodeKind // the glyph kind: step · child · wait
	Count    int                // >1: the ×N of the loop/collapse group
	Loop     bool               // the loop group: the loop arc
	Title    string             // the collapse group: the full member list
}

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

// measure sizes the canvas: colW is the horizontal pitch of a leaf column
// (the widest box + the column gap), rowH the vertical pitch of a depth
// row (the box height + the row gap); the canvas is the tight bounding box
// of the columns and the depth rows (plus the margins).
func (g *graphLayout) measure(roots []pathNode) (colW, rowH, maxW float64) {
	leaves := 0
	var m func(n pathNode, depth int)
	m = func(n pathNode, depth int) {
		if isTerminal(n) {
			return // the terminal marker: no box, no depth
		}
		if depth > g.maxDepth {
			g.maxDepth = depth
		}
		if n.Step != "" {
			if w := nodeBoxW(n.Step, n.Kind); w > maxW {
				maxW = w
			}
			kids := 0
			for _, c := range n.Children {
				if !isTerminal(c) {
					kids++
				}
			}
			if kids == 0 {
				leaves++
			}
		}
		for _, c := range n.Children {
			if n.Step == "" {
				m(c, depth) // the invisible group: the members don't consume a depth
			} else {
				m(c, depth+1)
			}
		}
	}
	for _, r := range roots {
		m(r, 0)
	}
	if leaves == 0 {
		leaves = 1 // no path: a valid (empty) canvas
	}
	colW = maxW + graphColGap
	rowH = graphNodeH + graphRowGap
	g.width = 2*graphMargin + (float64(leaves)-1)*colW + maxW
	g.height = 2*graphMargin + float64(g.maxDepth)*rowH + graphNodeH
	return colW, rowH, maxW
}

// place lays out the nodes: the leaves take columns (the slot x) and each
// parent centers on the average x of its children (the terminal marker
// aside); the depth runs top to bottom (y). The invisible group node
// consumes no depth: its members take the group's depth row, and the group
// centers on their x.
func (g *graphLayout) place(roots []pathNode, colW, rowH, maxW float64) []graphNode {
	slot := 0.0
	var place func(n pathNode, depth int) graphNode
	place = func(n pathNode, depth int) graphNode {
		node := graphNode{
			step:  n.Step,
			state: n.State,
			w:     nodeBoxW(n.Step, n.Kind),
			kind:  n.Kind,
			loop: n.Loop,
			title: n.Title,
		}
		node.y = graphMargin + float64(depth)*rowH + graphNodeH/2
		if n.Step == "" {
			// The invisible group: no box of its own.
			node.w = 0
			sum := 0.0
			for _, c := range n.Children {
				if isTerminal(c) {
					continue
				}
				child := place(c, depth)
				node.children = append(node.children, child)
				sum += child.x
			}
			if len(node.children) > 0 {
				node.x = sum / float64(len(node.children))
			}
			return node
		}
		kids := 0
		for _, c := range n.Children {
			if !isTerminal(c) {
				kids++
			}
		}
		if kids == 0 {
			node.x = graphMargin + slot*colW + maxW/2
			slot++
		} else {
			sum := 0.0
			for _, c := range n.Children {
				child := place(c, depth+1)
				node.children = append(node.children, child)
				if isTerminal(c) {
					continue
				}
				sum += child.x
			}
			node.x = sum / float64(kids)
		}
		return node
	}
	placed := make([]graphNode, 0, len(roots))
	for _, r := range roots {
		placed = append(placed, place(r, 0))
	}
	return placed
}

// collect appends the placed nodes to the layout (preorder — the document
// order is the execution order) and the edges, one per continuation
// (parent -> child): the invisible group relays its edges — the visible
// parent reaches each member directly.
func (g *graphLayout) collect(placed []graphNode) {
	var walk func(n graphNode)
	walk = func(n graphNode) {
		g.nodes = append(g.nodes, n)
		for _, c := range n.children {
			walk(c)
		}
	}
	var edges func(n graphNode)
	edges = func(n graphNode) {
		for _, c := range n.children {
			if c.step == "" {
				if len(c.children) == 0 {
					continue // the terminal marker
				}
				for _, m := range c.children {
					if n.step != "" {
						g.edges = append(g.edges, [2]graphNode{n, m})
					}
					edges(m)
				}
			} else {
				if n.step != "" {
					g.edges = append(g.edges, [2]graphNode{n, c})
				}
				edges(c)
			}
		}
	}
	for _, r := range placed {
		walk(r)
		edges(r)
	}
}

// svg emits the static SVG: edges under the nodes, one rect + label per
// step — the child/wait boxes carry their symbol, the loop group the arc,
// the collapse group its <title> member list — and the ∥ marks between the
// parallel group's member boxes. The colors come from the stylesheet
// (g-node/g-edge/g-text/g-arrow + the symbol classes).
func (g *graphLayout) svg() string {
	w, h := int(math.Round(g.width*graphScale)), int(math.Round(g.height*graphScale))
	var b strings.Builder
	fmt.Fprintf(&b, `<svg class="paths-graph" width="%d" height="%d" viewBox="0 0 %d %d" role="img" aria-label="%s"><g transform="scale(%s)">`,
		w, h, w, h, g.label, strconv.FormatFloat(graphScale, 'f', -1, 64))
	b.WriteString(arrowDefs)
	for _, e := range g.edges {
		fmt.Fprintf(&b, `<line class="g-edge" x1="%s" y1="%s" x2="%s" y2="%s" marker-end="url(#flowdeck-arrow)"/>`,
			num(e[0].x), num(e[0].y+graphNodeH/2), num(e[1].x), num(e[1].y-graphNodeH/2-1))
	}
	for _, n := range g.nodes {
		if n.step == "" {
			continue // the invisible group and the terminal marker: no box
		}
		cls := "g-node"
		if n.state != "" {
			cls += " " + n.state
		}
		if n.title != "" {
			fmt.Fprintf(&b, `<rect class="%s" x="%s" y="%s" width="%s" height="%s" rx="5"><title>%s</title></rect>`,
				cls, num(n.x-n.w/2), num(n.y-graphNodeH/2), num(n.w), num(graphNodeH), template.HTMLEscapeString(n.title))
		} else {
			fmt.Fprintf(&b, `<rect class="%s" x="%s" y="%s" width="%s" height="%s" rx="5"/>`,
				cls, num(n.x-n.w/2), num(n.y-graphNodeH/2), num(n.w), num(graphNodeH))
		}
		switch n.kind {
		case coregraph.NodeChild:
			// The child symbol (◉: ring + dot) in the box's right zone.
			sx := n.x + n.w/2 - graphPadX - graphSymW/2
			fmt.Fprintf(&b, `<g class="child-symbol" transform="translate(%s,%s)"><circle class="child-ring" r="5"/><circle class="child-dot" r="2"/></g>`,
				num(sx), num(n.y))
		case coregraph.NodeWait:
			// The wait symbol (…: three dots) in the box's right zone.
			sx := n.x + n.w/2 - graphPadX - graphSymW/2
			fmt.Fprintf(&b, `<g class="wait-symbol"><circle cx="%s" cy="%s" r="1.5"/><circle cx="%s" cy="%s" r="1.5"/><circle cx="%s" cy="%s" r="1.5"/></g>`,
				num(sx-6), num(n.y), num(sx), num(n.y), num(sx+6), num(n.y))
		}
		if n.loop {
			// The loop symbol in the box's top-right corner: an open 300°
			// arc with an arrowhead (the same glyph as the block outline).
			fmt.Fprintf(&b, `<g class="loop-symbol" transform="translate(%s,%s) scale(0.875)"><path d="M5.5,0 A5.5,5.5 0 1 1 2.75,-4.76" fill="none" stroke-width="2"/><polygon points="6.65,-2.51 4.25,-7.36 1.25,-2.16"/></g>`,
				num(n.x+n.w/2-7), num(n.y-graphNodeH/2+7))
		}
		labelX := n.x
		if n.kind == coregraph.NodeChild || n.kind == coregraph.NodeWait {
			labelX = n.x - graphSymW/2 // center the label in the text zone
		}
		fmt.Fprintf(&b, `<text class="g-text" x="%s" y="%s" text-anchor="middle">%s</text>`,
			num(labelX), num(n.y+5), template.HTMLEscapeString(n.step))
	}
	for _, n := range g.nodes {
		if n.step == "" && len(n.children) >= 2 {
			// The ∥ mark (async-symbol) between the sibling member boxes of
			// the parallel group.
			for i := 1; i < len(n.children); i++ {
				a, c := n.children[i-1], n.children[i]
				mx := (a.x + a.w/2 + c.x - c.w/2) / 2
				fmt.Fprintf(&b, `<g class="async-symbol" transform="translate(%s,%s)"><path d="M-3,-6 L-3,6 M3,-6 L3,6"/></g>`,
					num(mx), num(a.y))
			}
		}
	}
	b.WriteString(`</g></svg>`)
	return b.String()
}

// groupPath groups the executed steps into the path tree (pure — task
// 02): the consecutive rows of the repeated step name are the loop group
// (one box, ×N); the consecutive rows whose [StartedAt, CompletedAt]
// intervals overlap are the parallel group (the previous box forks the
// invisible group node — the continuation hangs off the last member); the
// rest are plain steps (the child/wait rows carry their glyph). A group of
// more than 4 members collapses into one annotated box (×N, the full
// member list in the box's <title>).
func groupPath(steps []reads.Step) []pathNode {
	var boxes []*pathNode
	i := 0
	for i < len(steps) {
		// The loop group: the consecutive rows of the repeated step name
		// (the loop body) — one box, ×N.
		j := i
		// The child rows keep their own names (each dispatch is its own
		// box) — only the step/wait rows loop.
		for j+1 < len(steps) && steps[j+1].Name == steps[i].Name && steps[i].Kind != reads.KindChild {
			j++
		}
		if j > i {
			n := groupBox(steps[i:j+1], true)
			boxes = append(boxes, &n)
			i = j + 1
			continue
		}
		// The parallel group: the consecutive rows whose [StartedAt,
		// CompletedAt] intervals overlap (runtime parallelism — the
		// fanout's go+select members).
		k := i
		for k+1 < len(steps) && intervalsOverlap(steps[k], steps[k+1]) {
			k++
		}
		if k > i {
			if k-i+1 > 4 {
				n := groupBox(steps[i:k+1], false)
				boxes = append(boxes, &n)
			} else {
				members := make([]pathNode, 0, k-i+1)
				for r := i; r <= k; r++ {
					members = append(members, stepNode(steps[r]))
				}
				if len(boxes) > 0 {
					// The previous box forks the invisible group node.
					fork := boxes[len(boxes)-1]
					fork.Children = []pathNode{{Children: members}}
				} else {
					// The group is first: the first member is the fork.
					fork := &members[0]
					fork.Children = []pathNode{{Children: members[1:]}}
					boxes = append(boxes, fork)
				}
				// The continuation hangs off the last member.
				boxes = append(boxes, &members[len(members)-1])
			}
			i = k + 1
			continue
		}
		n := stepNode(steps[i])
		boxes = append(boxes, &n)
		i++
	}
	if len(boxes) == 0 {
		return nil
	}
	// The continuations chain the boxes: the next box, and the invisible
	// terminal marker — the fork node's continuation is the group itself.
	terminal := pathNode{Terminal: true}
	for e := len(boxes) - 1; e >= 0; e-- {
		if len(boxes[e].Children) == 1 && boxes[e].Children[0].Step == "" {
			continue // the fork node
		}
		if e+1 < len(boxes) {
			boxes[e].Children = append(boxes[e].Children, *boxes[e+1])
		}
		boxes[e].Children = append(boxes[e].Children, terminal)
	}
	return []pathNode{*boxes[0]}
}

// groupBox is the group box: the loop group (the repeated name ×N — the
// loop arc when the members are ≤ 4, the annotated collapse beyond) and
// the collapse (more than 4 members — the ×N annotation, the full member
// list in the box's <title>). The box's badge state is the last member's
// (a child member's live state).
func groupBox(rows []reads.Step, loop bool) pathNode {
	last := rows[len(rows)-1]
	name, state := last.Name, string(last.State)
	if last.Kind == reads.KindChild && last.ChildName != "" {
		name, state = last.ChildName, string(last.ChildState)
	}
	allSame := true
	var names []string
	for _, r := range rows {
		if r.Name != rows[0].Name {
			allSame = false
		}
		names = append(names, r.Name)
	}
	n := pathNode{
		Step:  "×" + strconv.Itoa(len(rows)),
		State: state,
		Count: len(rows),
		Kind:  coregraph.NodeStep,
	}
	if allSame {
		n.Step = name + " ×" + strconv.Itoa(len(rows))
	}
	if len(rows) > 4 {
		n.Title = strings.Join(names, ", ")
	}
	if loop && len(rows) <= 4 {
		n.Loop = true
	}
	return n
}

// intervalsOverlap reports whether the two rows' [StartedAt, CompletedAt]
// intervals overlap (runtime parallelism — the fanout's go+select members);
// a row with a zero bound overlaps nothing.
func intervalsOverlap(a, b reads.Step) bool {
	if a.StartedAt.IsZero() || a.CompletedAt.IsZero() || b.StartedAt.IsZero() || b.CompletedAt.IsZero() {
		return false
	}
	return !a.StartedAt.After(b.CompletedAt) && !b.StartedAt.After(a.CompletedAt)
}

// stepNode is one plain row's box: the step · child · wait rows render
// with their glyph (the child box is the child's name + the child's live
// badge state — a child row missing the child's name falls back to the
// row name and the row's state).
func stepNode(s reads.Step) pathNode {
	n := pathNode{Step: s.Name, State: string(s.State), Kind: coregraph.NodeStep}
	switch s.Kind {
	case reads.KindChild:
		n.Kind = coregraph.NodeChild
		if s.ChildName != "" {
			n.Step, n.State = s.ChildName, string(s.ChildState)
		}
	case reads.KindWait:
		n.Kind = coregraph.NodeWait
	}
	return n
}

// ExecutedPathToSVG renders the executed path of a run (phase 4, STEP 3)
// as the same graph as the possible paths, with the steps that ran, in
// execution order — grouped (task 02): the parallel members fork the fork
// point (the ∥ mark between the member boxes), the loop body is one box
// with the ×N annotation, the child/wait rows carry their glyph, and the
// groups of more than 4 members collapse into one annotated box. It is
// separate from the boot map: the path comes from Postgres, the map from
// the code.
func ExecutedPathToSVG(steps []reads.Step) template.HTML {
	g := newGraphLayout(groupPath(steps))
	g.label = "executed path"
	return template.HTML(g.svg())
}
