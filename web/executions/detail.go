package executions

import (
	"net/http"

	"flowdeck/core"
	"flowdeck/core/reads"
	"flowdeck/web/render"
)

// executionDetail is the data for the execution detail full page: the
// badge (polling while live), the first execution date, the stored input
// values, the attempts, the children (task 03 — one row per child, the
// child list shadows the core field with the template view), and — on
// failure — the error and the failed step.
type executionDetail struct {
	reads.ExecutionDetail
	Badge    Badge
	Retries  []retry
	Steps    stepsView
	Children []childView
	Title    string
}

// childView is the template view of one child workflow of the run
// (task 03): the id, the child's name, the badge state as a string.
type childView struct {
	ID    string
	Name  string
	State string
}

// childViews maps the core child refs onto the template view.
func childViews(refs []reads.ChildRef) []childView {
	if len(refs) == 0 {
		return nil
	}
	out := make([]childView, 0, len(refs))
	for _, ref := range refs {
		out = append(out, childView{ID: ref.ID, Name: ref.Name, State: string(ref.State)})
	}
	return out
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
		render.Fail(w, "detail", err)
		return
	}
	render.Page(w, "execution-detail.html", executionDetail{
		ExecutionDetail: d.ExecutionDetail,
		Badge:           BadgeFor(id, d.ExecutionDetail.State),
		Retries:         retriesFor(d.Attempts),
		Steps:           newStepsView(id, d.RunPath),
		Children:        childViews(d.ExecutionDetail.Children),
		Title:           "Execution " + id,
	})
}
