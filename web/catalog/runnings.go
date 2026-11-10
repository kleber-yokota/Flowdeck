package catalog

import (
	"fmt"
	"net/http"
	"time"

	"flowdeck/core"
	"flowdeck/core/board"
	"flowdeck/web/executions/badge"
	"flowdeck/web/render"
)

// runningPage is the data for the runnings full page (the HTMX shell that
// polls the partial — the shell itself has no data).
type runningPage struct {
	Title string
}

// runningRow is one row of the runnings partial: a running run (the
// started time 15:04 and the elapsed, computed server-side per render —
// the poll repaints them).
type runningRow struct {
	Name    string
	ID      string
	Started string
	Elapsed string
}

// handleRunningPage renders the runnings full page (the shell: the list is
// the partial's 5s poll).
func handleRunningPage(w http.ResponseWriter) {
	render.Page(w, "runnings.html", runningPage{Title: "flowdeck · v6 · running"})
}

// handleRunningList renders the runnings partial (one row per running run
// — no dedup by workflow name · the SDK's desc order) with no-store (the
// 5s poll).
func handleRunningList(w http.ResponseWriter, p core.Port) {
	live, err := p.Living()
	if err != nil {
		render.Fail(w, "living", err)
		return
	}
	rows, err := runningRows(live, time.Now())
	if err != nil {
		render.Fail(w, "living", err)
		return
	}
	render.Partial(w, "runnings-list.html", rows)
}

// runningRows maps the Living read onto the partial's rows: the started
// time (15:04) and the elapsed (<60s "42s" · ≥60s "1m 07s") computed from
// the RFC3339 CreatedAt against now; a malformed CreatedAt is an error
// (not a guess).
func runningRows(runs []board.LiveRun, now time.Time) ([]runningRow, error) {
	rows := make([]runningRow, 0, len(runs))
	for _, run := range runs {
		created, err := time.Parse(time.RFC3339, run.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("running: created_at %q: %w", run.CreatedAt, err)
		}
		rows = append(rows, runningRow{
			Name:    run.Name,
			ID:      run.ID,
			Started: created.Format("15:04"),
			Elapsed: badge.ElapsedLabel(now.Sub(created)),
		})
	}
	return rows, nil
}
