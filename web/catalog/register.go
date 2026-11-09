package catalog

import (
	"net/http"

	"flowdeck/core"
)

// Register wires the catalog routes onto the mux: the dashboard (the
// top), the runnings page (the running runs), the queued page (the
// queued runs) and the workflow pages (the spec form and the possible
// paths).
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
	mux.HandleFunc("GET /runnings", func(w http.ResponseWriter, _ *http.Request) {
		handleRunningPage(w)
	})
	mux.HandleFunc("GET /runnings/list", func(w http.ResponseWriter, _ *http.Request) {
		handleRunningList(w, p)
	})
	mux.HandleFunc("GET /queued", func(w http.ResponseWriter, _ *http.Request) {
		handleQueuedPage(w)
	})
	mux.HandleFunc("GET /queued/list", func(w http.ResponseWriter, _ *http.Request) {
		handleQueuedList(w, p)
	})
	mux.HandleFunc("GET /failed", func(w http.ResponseWriter, _ *http.Request) {
		handleFailedPage(w)
	})
	mux.HandleFunc("GET /failed/list", func(w http.ResponseWriter, r *http.Request) {
		handleFailedList(w, r, p)
	})
	mux.HandleFunc("GET /workflows/{name}", func(w http.ResponseWriter, r *http.Request) {
		handleWorkflowPage(w, r, p)
	})
}
