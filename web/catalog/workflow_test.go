package catalog

import (
	"net/http"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/web/internal/webtest"
)

// The workflow console page (no runs) against the booted core: the cmd
// line with the source breadcrumb, the runline dispatch and the split —
// the runlines on the left, the possible-paths graph on the right. The
// runlines themselves (the paged runs and their actions) are the
// workflow_runs_test.go file's.

// The console dispatch (the runline form): the form as a command line
// (the prompt, one input per spec item, the run button), targeting the
// execrows container — a no-input workflow renders the bare runline
// (the zero payload).
func TestWorkflowPageRendersRunlineDispatch(t *testing.T) {
	srv := bootedTestServer(t)

	code, body := webtest.Get(t, srv.URL+"/workflows/noinput-test")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	for _, want := range []string{
		`class="runline"`,
		`hx-post="/run/noinput-test"`,
		`hx-target="#executions"`,
		`hx-swap="afterbegin"`,
		"➜",
		`class="run">run`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// A no-input workflow has no inputs (the bare runline).
	if strings.Contains(body, `<input`) {
		t.Errorf("no-input workflow page has inputs")
	}
}

// The console layout: the cmd line with the source breadcrumb, the split
// — the runlines on the left, the possible-paths graph on the right.
func TestWorkflowPageConsoleLayout(t *testing.T) {
	srv := bootedTestServer(t)

	code, body := webtest.Get(t, srv.URL+"/workflows/noinput-test")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	for _, want := range []string{
		`$ flowdeck workflows noinput-test`,
		`<span class="file">/noinput.go</span>`,
		`class="split"`,
		`class="split-left"`,
		`class="graph-side"`,
		`<div class="graph">`,
		"<svg",
		`steps are boxes · if forks the branches`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// The list (left) must come before the graph (right).
	iExec, iGraph := strings.Index(body, `id="executions"`), strings.Index(body, `<div class="graph">`)
	if iExec < 0 || iGraph < 0 || iExec > iGraph {
		t.Errorf("executions must come before the graph: executions at %d, graph at %d", iExec, iGraph)
	}
}

// A page for a workflow that is not registered is a clean 404 (the
// registry is the source — the graph and the booted registry both agree).
func TestWorkflowPageUnknownIs404(t *testing.T) {
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/workflows/ghost"); code != http.StatusNotFound {
		t.Errorf("status: got %d want %d", code, http.StatusNotFound)
	}
}

// The source breadcrumb: the workflow's source file from the boot map —
// the testworkflows source is a flat directory, so the page carries the
// file span (the bare file) and no dir span.
func TestWorkflowPageBreadcrumb(t *testing.T) {
	srv := bootedTestServer(t)
	code, body := webtest.Get(t, srv.URL+"/workflows/noinput-test")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	if !strings.Contains(body, `<span class="file">/noinput.go</span>`) {
		t.Errorf("page does not contain the file span %q", `<span class="file">/noinput.go</span>`)
	}
	if strings.Contains(body, `<span class="dir">`) {
		t.Errorf("flat source renders a dir span: %s", body)
	}
}
