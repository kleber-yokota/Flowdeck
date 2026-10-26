package core

import (
	"flowdeck/core/graph"
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
	List func() []Workflow
	// Graph is the possible-paths graph component (phase 4, STEP 2 — the
	// boot map: map lookup, no re-parse, no DB). Never nil: empty before
	// Boot.
	Graph *graph.PathGraph

	// Spec returns the parameter spec of a workflow (phase 5, BOOT): the
	// input and the steps' default values — the web renders the form on
	// top of the workflow page.
	Spec func(name string) (graph.Spec, error)
	// Executions lists the runs of a workflow (phase 3, LIST).
	Executions func(name string) ([]Execution, error)

	// Dispatch: start a run.
	// Enqueue dispatches a registered workflow with the clean input
	// (sanitized by the web against the workflow's spec — the clean input
	// is what goes on the DBOS queue) and returns the new execution
	// (id + state + created_at).
	Enqueue func(name string, payload any) (Execution, error)

	// Run tracking: follow a run (the 2s polls).
	// Status reports the state of a run (the read half of the dispatch,
	// polled every 2s by the badge).
	Status func(workflowID string) (status.State, error)
	// Steps returns the executed path of a run (phase 4, STEP 3: the steps
	// that ran, in execution order — polled every 2s until the run is
	// terminal).
	Steps func(workflowID string) ([]Step, error)
	// Detail reads one run for its page (phase 3, DETAIL).
	Detail func(workflowID string) (ExecutionDetail, error)

	// Resume re-enqueues a failed run by id (phase 5, RESUME — the same
	// stored input, restarting from the last completed step) and returns
	// the ACK (the same id, enqueued again).
	Resume func(workflowID string) (Execution, error)
}

// NewPort returns the port wired to the live core instance. The graph is
// captured at this call — wire the port after Boot; before it, the port
// carries the empty graph (Of returns nil).
func NewPort() Port {
	theCore.mu.RLock()
	g := theCore.paths
	theCore.mu.RUnlock()
	if g == nil {
		g = &graph.PathGraph{} // not booted yet: the empty graph — Of returns nil
	}
	return Port{
		List:       List,
		Graph:      g,
		Executions: Executions,
		Spec:       Spec,
		Enqueue:    Enqueue,
		Status:     Status,
		Steps:      Steps,
		Detail:     Detail,
		Resume:     Resume,
	}
}
