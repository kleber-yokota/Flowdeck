package executions

import (
	"net/http"
	"time"

	"flowdeck/core"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/web/render"
)

// badge is the data for badge.html (the 2s poll fragment) and for the
// execution rows: the state's class and label, the terminal flag (the poll
// stops) and, on the rows, the first execution date.
type Badge struct {
	WorkflowID string
	Class      string
	Label      string
	Terminal   bool
	CreatedAt  time.Time
}

// badgeLabels is the display label of each state (the badge text).
var badgeLabels = map[status.State]string{
	status.Queued:    "Queued",
	status.Running:   "Running",
	status.Success:   "Success",
	status.Error:     "Error",
	status.Cancelled: "Cancelled",
}

// badgeFor builds the badge of a state (the poll fragment).
func BadgeFor(id string, st status.State) Badge {
	return Badge{
		WorkflowID: id,
		Class:      string(st),
		Label:      badgeLabels[st],
		Terminal:   st.Terminal(),
	}
}

// row builds the execution row (badge + first execution date + id) — the
// dispatch ACK and the rows of the workflow page.
func Row(ex reads.Execution) Badge {
	b := BadgeFor(ex.ID, ex.State)
	b.CreatedAt = ex.CreatedAt
	return b
}

// handleBadge renders the badge partial for the 2s poll (phase 2): the
// live states (queued/running) carry the hx attributes that keep the poll
// running; the terminal badge is static and stops the poll.
func handleBadge(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	st, err := p.Status(id)
	if err != nil {
		render.Fail(w, "status", err)
		return
	}
	render.Partial(w, "badge.html", BadgeFor(id, st))
}
