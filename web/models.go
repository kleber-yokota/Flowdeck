package web

import (
	"html/template"
	"time"

	"flowdeck/core"
	"flowdeck/core/status"
	"flowdeck/web/graph"
)

// indexPage is the data for the index full page (the HTMX shell that loads
// the workflow list from the core registry).
type indexPage struct {
	Title string
}

// workflowPage is the data for the per-workflow full page: the dispatch
// button, the generated paths graph (phase 4, STEP 2, overview — the boot
// map) and the runs of the workflow (phase 3, LIST), newest first.
type workflowPage struct {
	Title      string
	Name       string
	Graph      template.HTML
	Executions []badge
}

// executionDetail is the data for the per-execution detail page (phase 3,
// DETAIL). core.ExecutionDetail is embedded so the template reads ID,
// Name, Attempts, Error and FailedStep directly. Steps is the executed
// path (phase 4, STEP 3): the steps that ran, in execution order.
type executionDetail struct {
	core.ExecutionDetail
	Title   string
	Badge   badge
	Retries []retry
	Steps   stepsView
}

// stepsView is the data for the executed-path fragment (phase 4, STEP 3):
// the SVG graph of the steps that ran in this run (execution order — the
// branch the run took), read per poll from Postgres.
type stepsView struct {
	WorkflowID string
	Terminal   bool
	Graph      template.HTML
}

// newStepsView builds the fragment data from the run's state and its
// steps: a terminal run stops the 2s poll (no hx attributes on the
// fragment — the same mechanism as the badge).
func newStepsView(id string, state status.State, steps []core.Step) stepsView {
	return stepsView{WorkflowID: id, Terminal: state.Terminal(), Graph: graph.ExecutedPathToSVG(steps)}
}

// badge is the data for the badge fragment (the 2s poll partial) and for
// the execution row fragments (the listing and the dispatch ACK).
type badge struct {
	WorkflowID string
	Class      string
	Label      string
	Terminal   bool
	// CreatedAt is the row's first execution date (listing + ACK rows);
	// the badge fragment itself never renders it.
	CreatedAt time.Time
}

// badgeLabels maps the core states onto the badge labels (the
// presentation of the four colors; the terminal rule lives on core.State).
var badgeLabels = map[status.State]string{
	status.Queued:  "Queued",
	status.Running: "Running",
	status.Success: "Success",
	status.Error:   "Failed",
}

func badgeFor(id string, state status.State) badge {
	label := string(state)
	if l, ok := badgeLabels[state]; ok {
		label = l
	}
	return badge{WorkflowID: id, Class: string(state), Label: label, Terminal: state.Terminal()}
}

// row maps one execution onto its listing row: the badge plus the first
// execution date (the dispatch ACK and the listing rows both carry it).
func row(e core.Execution) badge {
	b := badgeFor(e.ID, e.State)
	b.CreatedAt = e.CreatedAt
	return b
}

// retry is one row of the retries list: every retry is a failure recovered
// by the retry policy — the run itself is not a "retry", so a run
// without errors has zero. The current/final attempt is shown by the badge
// and, on failure, by the error section.
type retry struct {
	Number int
	Label  string
	Class  string
}

// retriesFor renders the retries list from the DBOS attempt count:
// "retries" counts the retries (attempts − 1) — a run without errors has
// zero, so the number never implies a failure. Each retry is a "failed
// (recovered)"; the final (or current) status is the badge itself.
func retriesFor(attempts int) []retry {
	retries := 0
	if attempts > 1 {
		retries = attempts - 1
	}
	rs := make([]retry, 0, retries)
	for i := 1; i <= retries; i++ {
		rs = append(rs, retry{Number: i, Label: "failed (recovered)", Class: string(status.Error)})
	}
	return rs
}
