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
	"slices"
	"time"

	"flowdeck/core"
)

//go:embed templates/*.html static/*
var staticFS embed.FS

var templates = template.Must(template.ParseFS(staticFS, "templates/*.html"))

// New builds the HTMX web server — an abstraction over the core.
// listWorkflows returns the workflows registered in the core (single
// source of truth); the web knows nothing about specific workflows.
// executions lists the runs of a workflow (phase 3, LIST).
// enqueue dispatches a registered workflow (core → DBOS queue) and returns
// the new execution (id + state + created_at). status reports the state of
// a run (the read half of the dispatch, polled every 2s by the badge).
// detail reads one run for its page (phase 3, DETAIL). ping reports whether
// the core is up (health endpoint).
func New(
	listWorkflows func() []core.Workflow,
	executions func(name string) ([]core.Execution, error),
	enqueue func(name string, payload any) (core.Execution, error),
	status func(workflowID string) (core.State, error),
	detail func(workflowID string) (core.ExecutionDetail, error),
	ping func() error,
) http.Handler {
	static, err := fs.Sub(staticFS, "static")
	if err != nil {
		panic(fmt.Sprintf("static: %v", err))
	}

	mux := http.NewServeMux()

	// Full page (serves the HTMX shell)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		page(w, "index.html", indexPage{Title: "FlowDeck"})
	})

	// HTMX partial: fragment with the workflows registered in the core
	mux.HandleFunc("GET /workflows", func(w http.ResponseWriter, _ *http.Request) {
		partial(w, "workflows.html", listWorkflows())
	})

	// Full page: one registered workflow — the dispatch button and the runs
	// of it (phase 3, LIST): every execution the core knows, newest first —
	// workflow ID + badge.
	mux.HandleFunc("GET /workflows/{name}", func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("name")
		if !registered(listWorkflows(), name) {
			http.Error(w, "unknown workflow", http.StatusNotFound)
			return
		}
		exs, err := executions(name)
		if err != nil {
			fail(w, "executions", err)
			return
		}
		rows := make([]badge, 0, len(exs))
		for _, e := range exs {
			rows = append(rows, row(e))
		}
		page(w, "workflow.html", workflowPage{Title: name + " · FlowDeck", Name: name, Executions: rows})
	})

	// HTMX partial: POST run — dispatch (phase 2). The core enqueues the run
	// on the DBOS queue; the response is the execution row fragment ("queued"
	// badge + visible workflow ID — the ACK), prepended to the executions
	// list on the workflow page.
	mux.HandleFunc("POST /run/{name}", func(w http.ResponseWriter, r *http.Request) {
		ex, err := enqueue(r.PathValue("name"), nil)
		if err != nil {
			fail(w, "enqueue", err)
			return
		}
		partial(w, "execution.html", row(ex))
	})

	// HTMX partial: the execution badge for the 2s poll (phase 2). Returns
	// only the badge fragment. Terminal badges carry no hx attributes, so
	// the swapped-in element has nothing left to re-trigger and the poll
	// stops.
	mux.HandleFunc("GET /status/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		state, err := status(id)
		if err != nil {
			fail(w, "status", err)
			return
		}
		partial(w, "badge.html", badgeFor(id, state))
	})

	// Full page: the detail of one execution (phase 3, DETAIL — click on the
	// workflow ID in the list): badge state, the attempts (retries 1..N-1
	// are the failures recovered by a retry, retry N the final status) and,
	// on failure, the error and the step that failed.
	mux.HandleFunc("GET /executions/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		d, err := detail(id)
		if err != nil {
			fail(w, "detail", err)
			return
		}
		page(w, "execution-detail.html", executionDetail{
			ExecutionDetail: d,
			Title:           d.ID + " · FlowDeck",
			Badge:           badgeFor(d.ID, d.State),
			Retries:         retriesFor(d.Attempts),
		})
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

// registered reports whether name is in the core's workflow registry (the
// single source of truth — the web never hardcodes a workflow).
func registered(workflows []core.Workflow, name string) bool {
	return slices.ContainsFunc(workflows, func(wf core.Workflow) bool { return wf.Name == name })
}

// row maps one execution onto its listing row: the badge plus the first
// execution date (the dispatch ACK and the listing rows both carry it).
func row(e core.Execution) badge {
	b := badgeFor(e.ID, e.State)
	b.CreatedAt = e.CreatedAt
	return b
}

// fail maps a core error onto the HTTP error response: 404 for a workflow
// or execution the core does not know, 503 while the core is not launched,
// and a logged 500 for anything unexpected.
func fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, core.ErrUnknownWorkflow):
		http.Error(w, "unknown workflow", http.StatusNotFound)
	case errors.Is(err, core.ErrUnknownExecution):
		http.Error(w, "unknown execution", http.StatusNotFound)
	case errors.Is(err, core.ErrNotLaunched):
		http.Error(w, "core not ready", http.StatusServiceUnavailable)
	default:
		slog.Error(op, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// page renders a full HTML page (a normal navigation, so the browser may
// cache the document as usual).
func page(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(templates, w, name, data)
}

// partial renders an HTMX fragment: the content is dynamic, so it must
// never be cached by the browser.
func partial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(templates, w, name, data)
}

// indexPage is the data for the index full page (the HTMX shell that loads
// the workflow list from the core registry).
type indexPage struct {
	Title string
}

// workflowPage is the data for the per-workflow full page: the dispatch
// button and the runs of the workflow (phase 3, LIST), newest first.
type workflowPage struct {
	Title      string
	Name       string
	Executions []badge
}

// executionDetail is the data for the per-execution detail page (phase 3,
// DETAIL). core.ExecutionDetail is embedded so the template reads ID,
// Name, Attempts, Error and FailedStep directly.
type executionDetail struct {
	core.ExecutionDetail
	Title   string
	Badge   badge
	Retries []retry
}

// retry is one row of the retries list: every retry is a failure recovered
// by the retry policy — the run itself is not a "retry", so a run
// without errors has zero. The current/final attempt is shown by the badge
// and, on failure, by the error section.
type retry struct {
	Number int
	Label  string
	Class  string
}

// badge is the data for the badge fragment (the 2s poll partial) and for
// the execution row fragments (the listing and the dispatch ACK).
type badge struct {
	WorkflowID string
	Class      string
	Label      string
	Terminal   bool
	// CreatedAt is the row's first execution date (listing + ACK rows);
	// the badge fragment itself never renders it.
	CreatedAt time.Time
}

// badgeStates maps the core states onto the badge presentation (the four
// colors of the dashboard; terminal states show a dot).
var badgeStates = map[core.State]struct {
	label    string
	terminal bool
}{
	core.StateQueued:  {label: "Queued"},
	core.StateRunning: {label: "Running"},
	core.StateSuccess: {label: "Success", terminal: true},
	core.StateError:   {label: "Failed", terminal: true},
}

// retriesFor renders the retries list from the DBOS attempt count:
// "retries" counts the retries (attempts − 1) — a run without errors has
// zero, so the number never implies a failure. Each retry is a "failed
// (recovered)"; the final (or current) status is the badge itself.
func retriesFor(attempts int) []retry {
	retries := 0
	if attempts > 1 {
		retries = attempts - 1
	}
	rs := make([]retry, 0, retries)
	for i := 1; i <= retries; i++ {
		rs = append(rs, retry{Number: i, Label: "failed (recovered)", Class: string(core.StateError)})
	}
	return rs
}

func badgeFor(id string, state core.State) badge {
	b := badge{WorkflowID: id, Class: string(state)}
	if meta, ok := badgeStates[state]; ok {
		b.Label, b.Terminal = meta.label, meta.terminal
	} else {
		b.Label = string(state)
	}
	return b
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
