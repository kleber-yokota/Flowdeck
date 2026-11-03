package graph

import (
	"html/template"
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

// groupPath groups the executed steps into the path tree (pure — task
// 02): the consecutive rows of the repeated step name are the loop group
// (one box, ×N); the consecutive rows whose [StartedAt, CompletedAt]
// intervals overlap are the parallel group (the previous box forks the
// invisible group node — the continuation hangs off the last member; the
// group is first: the group node is the root); the wait rows are the
// continuation of the steps they wait on (never parallel members — the
// fanout's select hangs below the go group); the rest are plain steps
// (the child/wait rows carry their glyph). A group of more than 4 members
// collapses into one annotated box (×N, the full member list in the box's
// <title>).
func groupPath(steps []reads.Step) []pathNode {
	boxes := make([]*pathNode, 0, len(steps))
	var root *pathNode
	for i := 0; i < len(steps); {
		var r *pathNode
		i, boxes, r = buildRow(i, steps, boxes)
		if r != nil {
			root = r
		}
	}
	chainContinuations(boxes)
	if root != nil {
		return []pathNode{*root}
	}
	if len(boxes) == 0 {
		return nil
	}
	return []pathNode{*boxes[0]}
}

// buildRow appends the boxes for the row at i (the loop group, the
// parallel group, or the plain step — the wait row is the plain box, the
// continuation of the steps it waits on) and returns the next index, the
// updated chain, and the group node root (the first parallel group — the
// group node is the root, never chained).
func buildRow(i int, steps []reads.Step, boxes []*pathNode) (int, []*pathNode, *pathNode) {
	if end := loopGroupEnd(i, steps); end > i {
		n := groupBox(steps[i:end], true)
		return end, append(boxes, &n), nil
	}
	if steps[i].Kind == reads.KindWait {
		n := stepNode(steps[i])
		return i + 1, append(boxes, &n), nil
	}
	if end := parallelGroupEnd(i, steps); end > i {
		if end-i <= 4 && len(boxes) == 0 {
			root, boxes := firstParallelGroup(steps, i, end)
			return end, boxes, root
		}
		return end, appendParallelGroup(boxes, steps, i, end), nil
	}
	n := stepNode(steps[i])
	return i + 1, append(boxes, &n), nil
}

// loopGroupEnd returns the exclusive end of the run of consecutive rows
// that repeat the step name of the row i (the loop body). The child rows
// keep their own names (each dispatch is its own box) — only the step and
// wait rows loop.
func loopGroupEnd(i int, steps []reads.Step) int {
	j := i
	for j+1 < len(steps) && steps[j+1].Name == steps[i].Name && steps[i].Kind != reads.KindChild {
		j++
	}
	if j == i {
		return i // no repetition: not a loop group
	}
	return j + 1
}

// parallelGroupEnd returns the exclusive end of the run of consecutive
// rows whose [StartedAt, CompletedAt] intervals overlap (runtime
// parallelism — the fanout's go members). The scan stops at the wait rows
// (select · recv · getResult are the continuation of the steps they wait
// on — never parallel members).
func parallelGroupEnd(i int, steps []reads.Step) int {
	k := i
	for k+1 < len(steps) && steps[k+1].Kind != reads.KindWait && intervalsOverlap(steps[k], steps[k+1]) {
		k++
	}
	if k == i {
		return i // no overlap: not a parallel group
	}
	return k + 1
}

// firstParallelGroup is the parallel group at the start of the path: there
// is no previous box to fork from — the group node is the root, holding
// every member box, and the continuation hangs off the last member (the
// group node itself is never chained).
func firstParallelGroup(steps []reads.Step, start, end int) (*pathNode, []*pathNode) {
	members := make([]pathNode, 0, end-start)
	for r := start; r < end; r++ {
		members = append(members, stepNode(steps[r]))
	}
	return &pathNode{Children: members}, []*pathNode{&members[len(members)-1]}
}

// appendParallelGroup appends the parallel group's boxes: more than 4
// members collapse into one annotated box; otherwise the previous box
// forks the invisible group node (holding every member), and the
// continuation hangs off the last member.
func appendParallelGroup(boxes []*pathNode, steps []reads.Step, start, end int) []*pathNode {
	if end-start > 4 {
		n := groupBox(steps[start:end], false)
		return append(boxes, &n)
	}
	members := make([]pathNode, 0, end-start)
	for r := start; r < end; r++ {
		members = append(members, stepNode(steps[r]))
	}
	// The previous box forks the invisible group node.
	boxes[len(boxes)-1].Children = []pathNode{{Children: members}}
	// The continuation hangs off the last member.
	return append(boxes, &members[len(members)-1])
}

// chainContinuations links the boxes: each box points to the next box and
// to the invisible terminal marker — the fork node (its only child is the
// invisible group node) keeps its group child.
func chainContinuations(boxes []*pathNode) {
	terminal := pathNode{Terminal: true}
	for e := len(boxes) - 1; e >= 0; e-- {
		if isForkNode(boxes[e]) {
			continue
		}
		if e+1 < len(boxes) {
			boxes[e].Children = append(boxes[e].Children, *boxes[e+1])
		}
		boxes[e].Children = append(boxes[e].Children, terminal)
	}
}

// isForkNode reports the fork node: its only child is the invisible group
// node (the parallel group's fork).
func isForkNode(n *pathNode) bool {
	return len(n.Children) == 1 && n.Children[0].Step == ""
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
