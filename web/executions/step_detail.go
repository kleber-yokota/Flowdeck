package executions

import (
	"net/http"

	"flowdeck/core"
	"flowdeck/web/render"
)

// handleStepDetail renders the step-detail fragment: the stored output or
// the error of one step of a run (the expand route — GET
// /executions/{id}/steps/{step}).
func handleStepDetail(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	step := r.PathValue("step")
	sd, err := p.StepDetail(id, step)
	if err != nil {
		render.Fail(w, "step-detail", err)
		return
	}
	render.Partial(w, "step-detail.html", sd)
}
