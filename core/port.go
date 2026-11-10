// Package core is the DBOS side of FlowDeck: the Port (the single contract
// the web talks to) and its implementation.
package core

import (
	"flowdeck/core/actions"
	"flowdeck/core/board"
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
	Catalog
	Runs
	Dispatch
	LiveBoard
}

type Catalog interface {
	// List returns the workflows registered in the core (single source of
	// truth); the web knows nothing about specific workflows.
	List() []registry.Workflow
	// Graph is the possible-paths graph (name → the block model: the stored
	// tree, the read-back paths, the steps) — the boot map: map lookup, no
	// re-parse, no DB. Never nil: empty before Boot.
	Graph() *graph.PathGraph
	// Spec returns the parameter spec of a workflow: the input and the
	// steps' default values — the web renders the form on top of the
	// workflow page.
	Spec(name string) (graph.Spec, error)
}

type Runs interface {
	// Status reports the state of a run (the read half of the dispatch,
	// polled every 2s by the badge).
	Status(id string) (status.State, error)
	// Steps returns the executed path of a run (the state and the steps
	// that ran, in execution order — polled every 2s until the run is
	// terminal; the state rides in the RunPath and is the poll control).
	Steps(id string) (reads.RunPath, error)
	// Detail reads the execution detail page in one read: the execution's
	// row (badge, created_at, attempts, inputs, error + the failed step)
	// and the executed path (the state + the steps that ran — the poll
	// control rides on it).
	Detail(id string) (reads.DetailPath, error)
	// StepDetail reads the output or error of one step row of a run (the
	// expand fragment's read).
	StepDetail(id, step string) (reads.StepDetail, error)
	// Executions lists one page of the runs of a workflow (the console):
	// limit rows from the offset (before), newest first, and the HasMore
	// flag (the pager — no total).
	Executions(name string, limit int, before int) (reads.ExecutionPage, error)
}

type Dispatch interface {
	// Enqueue dispatches a registered workflow with the clean input
	// (sanitized by the web against the workflow's spec — the clean input
	// is what goes on the DBOS queue) and returns the new execution
	// (id + state + created_at).
	Enqueue(name string, payload any) (reads.Execution, error)
	// Resume re-enqueues a failed run by id (the same stored input,
	// restarting from the last completed step) and returns the ACK (the
	// same id, enqueued again).
	Resume(id string) (reads.Execution, error)
	// Cancel cancels a live run by id (queued/running — the guard; a
	// terminal run is not cancellable) and returns the ACK (the same id,
	// now cancelled).
	Cancel(id string) (reads.Execution, error)
}

type LiveBoard interface {
	// Counts returns the three live buckets of the dashboard (the live
	// read): the runs running right now · the runs in the queue right now
	// (firing the same workflow several times counts each run) · the
	// workflows whose latest run is failed (error or cancelled) — a
	// latest success with no live run is in no box.
	Counts() (board.Live, error)
	// Living — the runnings list: one row per running run (no dedup by
	// workflow name), the SDK's desc order, not up → boot.ErrNotLaunched
	// (like every other read).
	Living() ([]board.LiveRun, error)
	// Queued — the queued list: one row per queued run (no dedup by
	// workflow name), the SDK's desc order, not up → boot.ErrNotLaunched
	// (like every other read). The mirror of Living (the ENQUEUED ∪ DELAYED
	// bucket).
	Queued() ([]board.LiveRun, error)
	// Failed — one row per failed workflow (the latest run error ∪
	// cancelled), with the error message (the batched error payload) —
	// core/board/failed.go. Not up → boot.ErrNotLaunched (like every other
	// read).
	Failed() ([]board.FailedRun, error)
}

type catalogAdapter struct{}

func (catalogAdapter) List() []registry.Workflow {
	return registry.List()
}

func (catalogAdapter) Graph() *graph.PathGraph {
	if g := boot.Paths(); g != nil {
		return g
	}
	return &graph.PathGraph{} // not booted yet: the empty graph — Of returns nil
}

func (catalogAdapter) Spec(name string) (graph.Spec, error) {
	return registry.Spec(name)
}

type runsAdapter struct{}

func (runsAdapter) Status(id string) (status.State, error) {
	return reads.Status(id)
}

func (runsAdapter) Steps(id string) (reads.RunPath, error) {
	return reads.Steps(id)
}

func (runsAdapter) Detail(id string) (reads.DetailPath, error) {
	return reads.Detail(id)
}

func (runsAdapter) StepDetail(id, step string) (reads.StepDetail, error) {
	return reads.StepDetailOf(id, step)
}

func (runsAdapter) Executions(name string, limit int, before int) (reads.ExecutionPage, error) {
	return reads.Executions(name, limit, before)
}

type dispatchAdapter struct{}

func (dispatchAdapter) Enqueue(name string, payload any) (reads.Execution, error) {
	return actions.Enqueue(name, payload)
}

func (dispatchAdapter) Resume(id string) (reads.Execution, error) {
	return actions.Resume(id)
}

func (dispatchAdapter) Cancel(id string) (reads.Execution, error) {
	return actions.Cancel(id)
}

type liveBoardAdapter struct{}

func (liveBoardAdapter) Counts() (board.Live, error) {
	return board.LiveCounts()
}

func (liveBoardAdapter) Living() ([]board.LiveRun, error) {
	return board.Living()
}

func (liveBoardAdapter) Queued() ([]board.LiveRun, error) {
	return board.Queued()
}

func (liveBoardAdapter) Failed() ([]board.FailedRun, error) {
	return board.Failed()
}

// NewPort returns the port wired to the live core instance. The graph
// resolves per call (Catalog.Graph() → boot.Paths(), nil → the empty
// graph).
func NewPort() Port {
	return Port{
		Catalog:   catalogAdapter{},
		Runs:      runsAdapter{},
		Dispatch:  dispatchAdapter{},
		LiveBoard: liveBoardAdapter{},
	}
}
