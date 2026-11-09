package catalog

import (
	"fmt"
	"net/http"
	"time"

	"flowdeck/core"
	"flowdeck/core/reads"
	"flowdeck/web/render"
)

// failedPage is the data for the failed full page (the HTMX shell that
// polls the partial — the shell itself has no data).
type failedPage struct {
	Title string
}

// failedRow is one row of the failed partial: a failed workflow (the
// latest run error ∪ cancelled) — the ts ("02/01 15:04") computed
// server-side per render from the RFC3339 CreatedAt, the status (the row's
// state) and the message (the error rows' error message · the batched
// error payload · the cancelled rows' empty message).
type failedRow struct {
	Name    string
	ID      string
	TS      string
	Status  string
	Message string
}

// failedList is the data for the failed partial: the rows of the page and
// the pager (the wall's pager shape — the wallPage reuse).
type failedList struct {
	Rows  []failedRow
	Start int
	End   int
	Total int
	Prev  int
	Next  int
}

// handleFailedPage renders the failed full page (the shell: the list is
// the partial's 5s poll).
func handleFailedPage(w http.ResponseWriter) {
	render.Page(w, "failed.html", failedPage{Title: "flowdeck · v6 · failed"})
}

// handleFailedList renders the failed partial (one row per failed
// workflow — the latest run error ∪ cancelled · the SDK's desc order) with
// no-store (the 5s poll).
func handleFailedList(w http.ResponseWriter, r *http.Request, p core.Port) {
	runs, err := p.Failed()
	if err != nil {
		render.Fail(w, "failed", err)
		return
	}
	rows, err := failedRows(runs, time.Now())
	if err != nil {
		render.Fail(w, "failed", err)
		return
	}
	if len(rows) == 0 {
		render.Partial(w, "failed-list.html", failedList{})
		return
	}
	start, end, prev, next := wallPage(len(rows), r.URL.Query().Get("page"))
	render.Partial(w, "failed-list.html", failedList{
		Rows:  rows[start:end],
		Start: start + 1,
		End:   end,
		Total: len(rows),
		Prev:  prev,
		Next:  next,
	})
}

// failedRows maps the Failed read onto the partial's rows: the ts
// ("02/01 15:04") from the RFC3339 CreatedAt (a malformed CreatedAt is an
// error — not a guess), the status and the message copied from the row.
func failedRows(runs []reads.FailedRun, now time.Time) ([]failedRow, error) {
	rows := make([]failedRow, 0, len(runs))
	for _, run := range runs {
		created, err := time.Parse(time.RFC3339, run.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed: created_at %q: %w", run.CreatedAt, err)
		}
		rows = append(rows, failedRow{
			Name:    run.Name,
			ID:      run.ID,
			TS:      created.Format("02/01 15:04"),
			Status:  run.State,
			Message: run.Message,
		})
	}
	return rows, nil
}
