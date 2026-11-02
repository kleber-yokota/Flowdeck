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
	"html/template"
	"io/fs"
	"net/http"

	"flowdeck/core"
)

// staticFS holds the two static trees: the templates (rendered by Go
// templates) and the vendored HTMX (served as-is).
//
//go:embed all:templates static
var staticFS embed.FS

// templates is the template set (templates/, one named template per file
// — rendered per request, no template cache).
var templates *template.Template

// static is the static filesystem (static/, the vendored HTMX and the
// stylesheet — served as-is, no CDN at runtime).
var static fs.FS

func init() {
	entries, err := fs.ReadDir(staticFS, "templates")
	if err != nil {
		panic("web: templates: " + err.Error())
	}
	templates = template.New("web")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := staticFS.ReadFile("templates/" + e.Name())
		if err != nil {
			panic("web: templates/" + e.Name() + ": " + err.Error())
		}
		template.Must(templates.New(e.Name()).Parse(string(b)))
	}
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
