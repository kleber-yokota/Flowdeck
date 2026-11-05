// Package core is the DBOS side of FlowDeck: the Port (the single contract
// the web talks to) and its implementation.
package core

import (
	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/graph"
	"flowdeck/core/reads"
	"flowdeck/core/registry"
	"flowdeck/core/status"
)

// Port is how the web talks to the core — the single contact point between
// the two halves of the process. The web never touches DBOS/Postgres:
// every registry read, dispatch and run tracking goes through here.
// (Core health is a different contact: Ready.)
type Port struct {
	// Registry: which workflows exist and what they could do.
	// List returns the workflows registered in the core (single source of
	// truth); the web knows nothing about specific workflows.
	List func() []registry.Workflow
	// Graph is the possible-paths graph (name → the block model: the stored
	// tree, the read-back paths, the steps) — the boot map: map lookup, no
	// re-parse, no DB. Never nil: empty before Boot.
	Graph *graph.PathGraph

	// Spec returns the parameter spec of a workflow (phase 5, BOOT): the
	// input and the steps' default values — the web renders the form on
	// top of the workflow page.
	Spec func(name string) (graph.Spec, error)
	// Executions lists the runs of a workflow (phase 3, LIST).
	Executions func(name string) ([]reads.Execution, error)

	// Dispatch: start a run.
	// Enqueue dispatches a registered workflow with the clean input
	// (sanitized by the web against the workflow's spec — the clean input
	// is what goes on the DBOS queue) and returns the new execution
	// (id + state + created_at).
	Enqueue func(name string, payload any) (reads.Execution, error)

	// Run tracking: follow a run (the 2s polls).
	// Status reports the state of a run (the read half of the dispatch,
	// polled every 2s by the badge).
	Status func(workflowID string) (status.State, error)
	// Steps returns the executed path of a run (phase 4, STEP 3: the state
	// and the steps that ran, in execution order — polled every 2s until
	// the run is terminal; the state rides in the RunPath and is the poll
	// control).
	Steps func(workflowID string) (reads.RunPath, error)
	// Detail reads the execution detail page in one read (phase 3, DETAIL ·
	// STEP 3): the execution's row (badge, created_at, attempts, inputs,
	// error + the failed step) and the executed path (the state + the steps
	// that ran — the poll control rides on it).
	Detail func(workflowID string) (reads.DetailPath, error)

	// Resume re-enqueues a failed run by id (phase 5, RESUME — the same
	// stored input, restarting from the last completed step) and returns
	// the ACK (the same id, enqueued again).
	Resume func(workflowID string) (reads.Execution, error)

	// Cancel cancels a live run by id (queued/running — the guard; a
	// terminal run is not cancellable) and returns the ACK (the same id,
	// now cancelled).
	Cancel func(workflowID string) (reads.Execution, error)

	// Live returns the three live buckets of the dashboard (the live
	// read): the runs running right now · the runs in the queue right now
	// (firing the same workflow several times counts each run) · the
	// workflows whose latest run is failed (error or cancelled) — a
	// latest success with no live run is in no box.
	Live func() (reads.Live, error)

	// Living — the runnings list: one row per running run (no dedup by
	// workflow name), the SDK's desc order, not up → boot.ErrNotLaunched
	// (like every other read).
	Living func() ([]reads.LiveRun, error)
}

// NewPort returns the port wired to the live core instance. The graph is
// captured at this call — wire the port after Boot; before it, the port
// carries the empty graph (Of returns nil).
func NewPort() Port {
	g := boot.Paths()
	if g == nil {
		g = &graph.PathGraph{} // not booted yet: the empty graph — Of returns nil
	}
	return Port{
		List:       registry.List,
		Graph:      g,
		Executions: reads.Executions,
		Spec:       registry.Spec,
		Enqueue:    actions.Enqueue,
		Status:     reads.Status,
		Steps:      reads.Steps,
		Detail:     reads.Detail,
		Resume:     actions.Resume,
		Cancel:     actions.Cancel,
		Live:       reads.LiveCounts,
		Living:     reads.Living,
	}
}
