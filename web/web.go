// Package web is the wiring of FlowDeck's HTMX presentation layer: the
// router (the catalog routes and the execution routes each register
// themselves in their subpackage), the healthz, the static assets, and
// the middlewares. It talks to the core through core.Port and to the
// graphs through core/graph and web/graph — it never touches DBOS/Postgres.
package web

import (
	"embed"
	"io/fs"
	"net/http"

	"flowdeck/core"
	"flowdeck/web/catalog"
	"flowdeck/web/executions"
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

// New wires the router: the catalog routes (the dashboard and the
// workflow pages) and the execution routes (the dispatch, the badge, the
// detail, the steps, the resume and the cancel) each register themselves
// (their own subpackage), the healthz goes through the injected readiness
// check (core.Ready in production — the same single contact), and the
// static assets are served as-is.
func New(p core.Port, ready func() error) http.Handler {
	mux := http.NewServeMux()
	catalog.Register(mux, p)
	executions.Register(mux, p)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		handleHealthz(w, ready)
	})
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))
	return withRecovery(withRequestLog(mux))
}
