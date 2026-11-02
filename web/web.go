// Package web is FlowDeck's HTMX presentation layer: it renders the
// dashboard (the workflows list, from the core registry), the workflow
// pages (the spec form, the possible paths), the dispatch (POST /run), the
// badge (the 2s poll), the execution detail (badge + executed path +
// resume/cancel), and the healthz. It talks to the core through core.Port
// and to the graphs through core/graph and web/graph — it never touches
// DBOS/Postgres.
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"flowdeck/core"
)

// staticFS holds the vendored HTMX and the stylesheet (served as-is, no
// CDN at runtime). The templates live in web/render (their own embed).
//
//go:embed static
var staticFS embed.FS

// static is the static filesystem (served as-is, no CDN at runtime).
var static fs.FS

func init() {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic("web: static fs: " + err.Error())
	}
	static = sub
}

// New wires the router: every route is wired here and nowhere else (the
// handlers in this package), the domain conversation goes through the
// injected Port, and the healthz goes through the injected readiness check
// (core.Ready in production — the same single contact).
func New(p core.Port, ready func() error) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		handleDashboard(w)
	})
	mux.HandleFunc("GET /workflows", func(w http.ResponseWriter, _ *http.Request) {
		handleWorkflowsList(w, p)
	})
	mux.HandleFunc("GET /workflows/{name}", func(w http.ResponseWriter, r *http.Request) {
		handleWorkflowPage(w, r, p)
	})
	mux.HandleFunc("POST /run/{name}", func(w http.ResponseWriter, r *http.Request) {
		handleRunDispatch(w, r, p)
	})
	mux.HandleFunc("GET /status/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleBadge(w, r, p)
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
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		handleHealthz(w, ready)
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	return withRecovery(withRequestLog(mux))
}
