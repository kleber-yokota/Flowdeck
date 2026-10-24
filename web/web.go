package web

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"flowdeck/core"
)

//go:embed templates/*.html static/*
var staticFS embed.FS

var templates = template.Must(template.ParseFS(staticFS, "templates/*.html"))

// New builds the HTMX web server — an abstraction over the core.
// listWorkflows returns the workflows registered in the core (single
// source of truth); the web knows nothing about specific workflows.
// enqueue dispatches a registered workflow (core → DBOS queue) and returns
// the new workflow ID. ping reports whether the core is up (health endpoint).
func New(listWorkflows func() []core.Workflow, enqueue func(name string, payload any) (string, error), ping func() error) http.Handler {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(fmt.Sprintf("static: %v", err))
	}

	mux := http.NewServeMux()

	// Full page (serves the HTMX shell)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		render(templates, w, "index.html", nil)
	})

	// HTMX partial: fragment with the workflows registered in the core
	mux.HandleFunc("GET /workflows", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		render(templates, w, "workflows.html", listWorkflows())
	})

	// Full page: one registered workflow — the dispatch button and the list
	// of executions (workflow IDs) triggered from it.
	mux.HandleFunc("GET /workflows/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		registered := false
		for _, wf := range listWorkflows() {
			if wf.Name == name {
				registered = true
				break
			}
		}
		if !registered {
			http.Error(w, "unknown workflow", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		render(templates, w, "workflow.html", workflowPage{Name: name})
	})

	// HTMX partial: POST run — dispatch (phase 2). The core enqueues the run
	// on the DBOS queue; the response is the execution row fragment ("queued"
	// badge + visible workflow ID — the ACK), prepended to the executions
	// list on the workflow page.
	mux.HandleFunc("POST /run/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		id, err := enqueue(name, nil)
		if err != nil {
			if errors.Is(err, core.ErrUnknownWorkflow) {
				http.Error(w, "unknown workflow", http.StatusNotFound)
				return
			}
			slog.Error("enqueue", "workflow", name, "err", err)
			http.Error(w, "enqueue failed", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		render(templates, w, "execution.html", runBadge{WorkflowID: id})
	})

	// Health: the web pings the core; the container healthcheck pings this
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		if err := ping(); err != nil {
			http.Error(w, "core not ready", http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		_, _ = w.Write([]byte("ok"))
	})

	// Vendored static assets (htmx) — no CDN at runtime
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(static)))

	return withRecovery(withRequestLog(mux))
}

// workflowPage is the data for the per-workflow full page.
type workflowPage struct {
	Name string
}

// runBadge is the data for the execution row fragment rendered after a
// dispatch.
type runBadge struct {
	WorkflowID string
}

// render executes the template into a buffer first, so a render error can
// never leave a partial 200 body in flight, and is always logged.
func render(t *template.Template, w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("render template", "template", name, "err", err)
		http.Error(w, "error rendering", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(buf.Bytes())
}

// withRecovery turns a handler panic into a logged 500 instead of a bare
// "http: panic serving" line with no useful context.
func withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic in handler", "path", r.URL.Path, "panic", rec)
				http.Error(w, "internal error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// withRequestLog logs each request: method, path, status, duration.
func withRequestLog(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		status := rec.status
		if status == 0 {
			status = http.StatusOK
		}
		slog.Info("http",
			"method", r.Method, "path", r.URL.Path,
			"status", status, "duration", time.Since(start).String())
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
