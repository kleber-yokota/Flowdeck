package catalog

import (
	"fmt"
	"net/http"
	"time"

	"flowdeck/core"
	"flowdeck/core/reads"
	"flowdeck/web/executions/badge"
	"flowdeck/web/render"
)

// queuedPage is the data for the queued full page (the HTMX shell that
// polls the partial — the shell itself has no data).
type queuedPage struct {
	Title string
}

// queuedRow is one row of the queued partial: a queued run (the queued
// since 15:04 and the wait, computed server-side per render — the poll
// repaints them).
type queuedRow struct {
	Name    string
	ID      string
	Started string
	Elapsed string
}

// handleQueuedPage renders the queued full page (the shell: the list is
// the partial's 5s poll).
func handleQueuedPage(w http.ResponseWriter) {
	render.Page(w, "queued.html", queuedPage{Title: "flowdeck · v6 · queued"})
}

// handleQueuedList renders the queued partial (one row per queued run —
// no dedup by workflow name · the SDK's desc order) with no-store (the
// 5s poll).
func handleQueuedList(w http.ResponseWriter, p core.Port) {
	runs, err := p.Queued()
	if err != nil {
		render.Fail(w, "queued", err)
		return
	}
	rows, err := queuedRows(runs, time.Now())
	if err != nil {
		render.Fail(w, "queued", err)
		return
	}
	render.Partial(w, "queued-list.html", rows)
}

// queuedRows maps the Queued read onto the partial's rows: the queued
// since time (15:04) and the wait (<60s "42s" · ≥60s "1m 07s") computed
// from the RFC3339 CreatedAt against now; a malformed CreatedAt is an
// error (not a guess).
func queuedRows(runs []reads.LiveRun, now time.Time) ([]queuedRow, error) {
	rows := make([]queuedRow, 0, len(runs))
	for _, run := range runs {
		created, err := time.Parse(time.RFC3339, run.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("queued: created_at %q: %w", run.CreatedAt, err)
		}
		rows = append(rows, queuedRow{
			Name:    run.Name,
			ID:      run.ID,
			Started: created.Format("15:04"),
			Elapsed: badge.ElapsedLabel(now.Sub(created)),
		})
	}
	return rows, nil
}
