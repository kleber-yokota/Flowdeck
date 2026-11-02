package web

import (
	"html/template"
	"net/http"

	"flowdeck/core"
	"flowdeck/web/graph"
)

// stepsView is the data for the steps partial (the executed path, polled
// every 2s until the run is terminal).
type stepsView struct {
	WorkflowID string
	Terminal   bool
	Graph      template.HTML
}

// newStepsView builds the steps partial of a run path (the steps that ran,
// in execution order — the branch the run took).
func newStepsView(workflowID string, path core.RunPath) stepsView {
	return stepsView{
		WorkflowID: workflowID,
		Terminal:   path.State.Terminal(),
		Graph:      graph.ExecutedPathToSVG(path.Steps),
	}
}

// handleExecutionSteps renders the steps partial (phase 4, STEP 3,
// polling): a live run returns the executed-path SVG with the hx
// attributes that keep the 2s poll running; a terminal run stops the poll.
func handleExecutionSteps(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	path, err := p.Steps(id)
	if err != nil {
		fail(w, "steps", err)
		return
	}
	partial(w, "steps.html", newStepsView(id, path))
}
