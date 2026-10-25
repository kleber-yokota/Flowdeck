// Package web is the HTMX web server — the presentation layer over the
// core: full pages plus HTMX fragments, core errors mapped onto HTTP,
// and the dispatch POST that enqueues a run on the core.
package web

import (
	"embed"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"slices"

	"flowdeck/core"
)

//go:embed templates/*.html static/*
var staticFS embed.FS

var templates = template.Must(template.ParseFS(staticFS, "templates/*.html"))

// New builds the HTMX web server — an abstraction over the core.
// listWorkflows returns the workflows registered in the core (single
// source of truth); the web knows nothing about specific workflows.
// paths reports the possible paths of a workflow (phase 4, STEP 2 — the
// boot map: map lookup, no re-parse, no DB).
// executions lists the runs of a workflow (phase 3, LIST).
// enqueue dispatches a registered workflow (core → DBOS queue) and returns
// the new execution (id + state + created_at). status reports the state of
// a run (the read half of the dispatch, polled every 2s by the badge).
// steps returns the executed path of a run (phase 4, STEP 3: the steps that
// ran, in execution order — polled every 2s until the run is terminal).
// detail reads one run for its page (phase 3, DETAIL). ping reports whether
// the core is up (health endpoint).
func New(
	listWorkflows func() []core.Workflow,
	paths func(name string) [][]string,
	executions func(name string) ([]core.Execution, error),
	enqueue func(name string, payload any) (core.Execution, error),
	status func(workflowID string) (core.State, error),
	steps func(workflowID string) ([]core.Step, error),
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

	// Full page: one registered workflow — the dispatch button, the
	// possible paths (phase 4, STEP 2, overview) and the runs of it
	// (phase 3, LIST): every execution the core knows, newest first —
	// workflow ID + badge.
	mux.HandleFunc("GET /workflows/{name}", func(w http.ResponseWriter, r *http.Request) {
		handleWorkflowPage(w, r, listWorkflows, paths, executions)
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

	// Full page: the detail of one execution (phase 3, DETAIL).
	mux.HandleFunc("GET /executions/{id}", func(w http.ResponseWriter, r *http.Request) {
		handleExecutionDetail(w, r, detail, steps)
	})

	// HTMX partial: the executed path of a run (phase 4, STEP 3, polling).
	mux.HandleFunc("GET /executions/{id}/steps", func(w http.ResponseWriter, r *http.Request) {
		handleExecutionSteps(w, r, status, steps)
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

// handleWorkflowPage renders the full page of one registered workflow:
// the dispatch button, the possible paths (phase 4, STEP 2, overview) and
// the runs of it (phase 3, LIST): every execution the core knows, newest
// first — workflow ID + badge.
func handleWorkflowPage(w http.ResponseWriter, r *http.Request,
	listWorkflows func() []core.Workflow,
	paths func(name string) [][]string,
	executions func(name string) ([]core.Execution, error)) {
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
	// Phase 4, STEP 2: the paths come from the boot map (PATHS: map
	// lookup, no re-parse, no DB) — never hardcoded in the web.
	page(w, "workflow.html", workflowPage{
		Title:      name + " · FlowDeck",
		Name:       name,
		Graph:      pathsToSVG(paths(name)),
		Executions: rows,
	})
}

// handleExecutionDetail renders the full page of one execution (phase 3,
// DETAIL — click on the workflow ID in the list): badge state, the
// attempts (retries 1..N-1 are the failures recovered by a retry, retry N
// the final status), on failure the error and the step that failed, and
// the executed path (phase 4, STEP 3: the steps that ran, in execution
// order).
func handleExecutionDetail(w http.ResponseWriter, r *http.Request,
	detail func(workflowID string) (core.ExecutionDetail, error),
	steps func(workflowID string) ([]core.Step, error)) {
	id := r.PathValue("id")
	d, err := detail(id)
	if err != nil {
		fail(w, "detail", err)
		return
	}
	exSteps, err := steps(id)
	if err != nil {
		fail(w, "steps", err)
		return
	}
	page(w, "execution-detail.html", executionDetail{
		ExecutionDetail: d,
		Title:           d.ID + " · FlowDeck",
		Badge:           badgeFor(d.ID, d.State),
		Retries:         retriesFor(d.Attempts),
		Steps:           newStepsView(id, d.State, exSteps),
	})
}

// handleExecutionSteps renders the HTMX partial of the executed path of a
// run (phase 4, STEP 3, polling): the SVG of the steps that ran, in
// execution order (the branch the run took), read from Postgres per poll —
// separate from the boot map of possible paths. Polled every 2s while the
// run is live; the fragment drops the hx attributes when the run is
// terminal and the poll stops.
func handleExecutionSteps(w http.ResponseWriter, r *http.Request,
	status func(workflowID string) (core.State, error),
	steps func(workflowID string) ([]core.Step, error)) {
	id := r.PathValue("id")
	state, err := status(id)
	if err != nil {
		fail(w, "status", err)
		return
	}
	exSteps, err := steps(id)
	if err != nil {
		fail(w, "steps", err)
		return
	}
	partial(w, "steps.html", newStepsView(id, state, exSteps))
}

// registered reports whether name is in the core's workflow registry (the
// single source of truth — the web never hardcodes a workflow).
func registered(workflows []core.Workflow, name string) bool {
	return slices.ContainsFunc(workflows, func(wf core.Workflow) bool { return wf.Name == name })
}
