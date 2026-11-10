package badge

import (
	"fmt"
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

// Runline is the runline row of the workflow console: the badge (the poll
// fragment), the elapsed (now − created), and — for the terminal rows —
// the last step and the exit (the decoded output on success, the error
// message on error), plus the row actions (cancel on a live run, resume
// on an error run).
type Runline struct {
	Badge
	Elapsed    string
	Step       string
	Exit       string
	Cancelable bool
	Resumeable bool
}

// Row builds the runline row of a listed run: the badge of its state and
// the elapsed (now − created); the terminal fields (the step and the
// exit) are enriched by the page for the terminal rows only.
func Row(ex reads.Execution) Runline {
	return Runline{
		Badge:      BadgeFor(ex.ID, ex.State),
		Elapsed:    ElapsedLabel(time.Now().Sub(ex.CreatedAt)),
		Cancelable: !ex.State.Terminal(),
		Resumeable: ex.State == status.Error,
	}
}

// ElapsedLabel renders the elapsed time: <60s "42s" · ≥60s "%dm %02ds"
// (moved from web/catalog — the console rows and the runnings/queued
// lists share it).
func ElapsedLabel(d time.Duration) string {
	seconds := int(d / time.Second)
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	return fmt.Sprintf("%dm %02ds", seconds/60, seconds%60)
}

// handleBadge renders the badge partial for the 2s poll (phase 2): the
// live states (queued/running) carry the hx attributes that keep the poll
// running; the terminal badge is static and stops the poll.
func HandleBadge(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	st, err := p.Status(id)
	if err != nil {
		render.Fail(w, "status", err)
		return
	}
	render.Partial(w, "badge.html", BadgeFor(id, st))
}
