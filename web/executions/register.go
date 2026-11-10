package executions

import (
	"net/http"

	"flowdeck/core"
	"flowdeck/web/executions/badge"
)

// Register wires the execution routes onto the mux: the dispatch (POST
// /run), the badge (the 2s poll), the detail (the read model), the steps
// (the executed path poll), the resume and the cancel.
func Register(mux *http.ServeMux, p core.Port) {
	mux.HandleFunc("POST /run/{name}", func(w http.ResponseWriter, r *http.Request) {
		handleRunDispatch(w, r, p)
	})
	mux.HandleFunc("GET /status/{id}", func(w http.ResponseWriter, r *http.Request) {
		badge.HandleBadge(w, r, p)
	})
	mux.HandleFunc("GET /executions/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleExecutionDetail(w, r, p)
	})
	mux.HandleFunc("GET /executions/{id}/steps", func(w http.ResponseWriter, r *http.Request) {
		handleExecutionSteps(w, r, p)
	})
	mux.HandleFunc("POST /resume/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleResume(w, r, p)
	})
	mux.HandleFunc("POST /cancel/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleCancel(w, r, p)
	})
}
