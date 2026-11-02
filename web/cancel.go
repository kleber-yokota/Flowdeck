package web

import (
	"net/http"

	"flowdeck/core"
)

// handleCancel cancels a live run by id (the cancel sequence): the SDK
// flips the row to CANCELLED and takes it out of the queue; the ACK badge
// (cancelled, terminal) replaces the badge and the 2s poll stops.
func handleCancel(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	ex, err := p.Cancel(id)
	if err != nil {
		fail(w, "cancel", err)
		return
	}
	partial(w, "badge.html", badgeFor(ex.ID, ex.State))
}
