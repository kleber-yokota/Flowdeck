package executions

import (
	"net/http"

	"flowdeck/core"
	"flowdeck/web/executions/badge"
	"flowdeck/web/render"
)

// handleResume re-enqueues a failed run by id (phase 5, RESUME) — the same
// stored input; the ACK badge replaces the badge (the 2s poll takes over).
func handleResume(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	ex, err := p.Resume(id)
	if err != nil {
		render.Fail(w, "resume", err)
		return
	}
	render.Partial(w, "badge.html", badge.BadgeFor(ex.ID, ex.State))
}
