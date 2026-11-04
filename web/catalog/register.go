package catalog

import (
	"net/http"

	"flowdeck/core"
)

// Register wires the catalog routes onto the mux: the dashboard (the
// top) and the workflow pages (the spec form and the possible paths).
func Register(mux *http.ServeMux, p core.Port) {
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		handleDashboard(w)
	})
	mux.HandleFunc("GET /workflows", func(w http.ResponseWriter, r *http.Request) {
		handleWorkflowsList(w, r, p)
	})
	mux.HandleFunc("GET /live", func(w http.ResponseWriter, _ *http.Request) {
		handleLive(w, p)
	})
	mux.HandleFunc("GET /workflows/{name}", func(w http.ResponseWriter, r *http.Request) {
		handleWorkflowPage(w, r, p)
	})
}
