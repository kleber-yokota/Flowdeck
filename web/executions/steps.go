package executions

import (
	"html/template"
	"net/http"
	"time"

	"flowdeck/core"
	"flowdeck/core/reads"
	"flowdeck/web/render"
)

// stepsView is the data for the steps partial (the v6.3 split fragment,
// polled every 2s until the run is terminal): the meta row (created, the
// live elapsed, the stored inputs, the retries count), the step log rows
// (with the child blocks and the dashed tail while live), and the
// executed-path graph (the dashed tail while live).
type stepsView struct {
	WorkflowID string
	Terminal   bool
	State      string // string(d.State) — the resume condition (error | cancelled)
	CreatedAt  time.Time
	Elapsed    string // badge.ElapsedLabel(now − created)
	Inputs     []reads.InputValue
	Retries    int // d.Attempts − 1 (the attempts floor at 1, so ≥ 0)
	Rows       []stepRowView
	Graph      template.HTML // executed.ExecutedPathToSVG(d.RunPath.Steps, !Terminal)
}

// handleExecutionSteps renders the steps partial (the v6.3 split,
// polling): the per-poll detail read (the meta needs the inputs, the
// created date and the attempts — the same class of read as today's steps
// read; the badge poll stays cheap), one steps read per child, and the
// shared builder (the page and the fragment carry the same data).
func handleExecutionSteps(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	d, err := p.Detail(id)
	if err != nil {
		render.Fail(w, "steps", err)
		return
	}
	childPaths, err := childPathsOf(p, d)
	if err != nil {
		render.Fail(w, "steps", err)
		return
	}
	render.Partial(w, "steps.html", newStepsView(id, d, childPaths))
}
