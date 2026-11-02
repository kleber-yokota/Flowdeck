package web

import (
	"net/http"

	"flowdeck/core"
)

// executionDetail is the data for the execution detail full page: the
// badge (polling while live), the first execution date, the stored input
// values, the attempts, and — on failure — the error and the failed step.
type executionDetail struct {
	core.ExecutionDetail
	Badge   badge
	Retries []retry
	Steps   stepsView
	Title   string
}

// retry is one row of the attempts list (attempts − 1: the recovered
// failures — the final attempt is the badge, not a row).
type retry struct {
	Number int
	Label  string
	Class  string
}

// retriesFor builds the attempts rows: attempts − 1 (the final attempt is
// the badge, not a row).
func retriesFor(attempts int) []retry {
	out := make([]retry, 0, attempts-1)
	for i := 1; i < attempts; i++ {
		out = append(out, retry{Number: i, Label: "failed (recovered)", Class: "recovered"})
	}
	return out
}

// handleExecutionDetail renders the execution detail full page (phase 3,
// DETAIL): the badge, the first execution date, the stored input values,
// the attempts, the executed path (polling while live) — and, on failure,
// the error and the failed step with the Resume button.
func handleExecutionDetail(w http.ResponseWriter, r *http.Request, p core.Port) {
	id := r.PathValue("id")
	d, err := p.Detail(id)
	if err != nil {
		fail(w, "detail", err)
		return
	}
	path, err := p.Steps(id)
	if err != nil {
		fail(w, "steps", err)
		return
	}
	page(w, "execution-detail.html", executionDetail{
		ExecutionDetail: d,
		Badge:           badgeFor(id, d.State),
		Retries:         retriesFor(d.Attempts),
		Steps:           newStepsView(id, path),
		Title:           "Execution " + id,
	})
}
