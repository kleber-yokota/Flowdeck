package catalog

import (
	"net/http"

	"flowdeck/core"
	"flowdeck/web/render"
)

// indexPage is the data for the index full page (the HTMX shell that loads
// the workflow list from the core registry).
type indexPage struct {
	Title string
}

// handleDashboard renders the dashboard full page (the HTMX shell).
func handleDashboard(w http.ResponseWriter) {
	render.Page(w, "index.html", indexPage{Title: "FlowDeck"})
}

// handleWorkflowsList renders the workflows partial (the dashboard list —
// from the core registry, no hardcoding; the empty state when the registry
// is empty or the core is not booted).
func handleWorkflowsList(w http.ResponseWriter, p core.Port) {
	render.Partial(w, "workflows.html", p.List())
}
