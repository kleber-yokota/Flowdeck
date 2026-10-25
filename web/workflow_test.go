package web

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
)

// graphNodeY matches a graph node rect: the node's y coordinate is group 1.
var graphNodeY = regexp.MustCompile(`class="g-node" x="[0-9.]+" y="([0-9.]+)"`)

// The workflow page (full page, opened from the list) has the dispatch
// button and the executions list that receives the ACK rows.
func TestWorkflowPageRendersDispatchButtonAndList(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	srv := httptest.NewServer(New(fakeLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows/wf-alpha")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`hx-post="/run/wf-alpha"`,
		`id="executions"`,
		"Run",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// okPaths returns no paths: no graph nodes are rendered.
	if strings.Contains(body, `class="g-node"`) {
		t.Error("graph nodes rendered without paths")
	}
}

// The workflow page layout (phase 4): the executions list (what was
// executed) is on the left and the generated paths graph is on the right.
func TestWorkflowPageColumnsLayout(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	fakePaths := func(name string) [][]string {
		return [][]string{{"one", "two"}}
	}
	srv := httptest.NewServer(New(fakeLister, fakePaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows/wf-alpha")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	iExec, iGraph := strings.Index(body, `id="executions"`), strings.Index(body, `class="graph"`)
	if iExec < 0 || iGraph < 0 || iExec > iGraph {
		t.Errorf("executions must come before the graph: executions at %d, graph at %d", iExec, iGraph)
	}
}

// The workflow page renders the possible paths (phase 4, STEP 2) as a
// graph: the common prefix is drawn once and the if forks the branches —
// not one row per path (from the boot map: map lookup, no re-parse, no DB).
func TestWorkflowPageRendersForkedGraph(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	fakePaths := func(name string) [][]string {
		return [][]string{{"base", "left"}, {"base", "right"}}
	}
	srv := httptest.NewServer(New(fakeLister, fakePaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows/wf-alpha")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	// The prefix once — the fork shows both branches, not one row per path.
	if n := strings.Count(body, ">base<"); n != 1 {
		t.Errorf("prefix drawn %d times, want 1", n)
	}
	for _, want := range []string{
		`<svg`,
		">left<",
		">right<",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// The fork: the branches sit on different rows than the prefix.
	if n := len(rowYs(t, body)); n < 2 {
		t.Errorf("fork rows: got %d distinct rows, want >= 2", n)
	}
}

// A workflow without an if renders a single linear row (all the nodes on
// the same row).
func TestWorkflowPageRendersLinearGraph(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	fakePaths := func(name string) [][]string {
		return [][]string{{"fetch", "parse", "store"}}
	}
	srv := httptest.NewServer(New(fakeLister, fakePaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows/wf-alpha")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if n := strings.Count(body, `class="g-node"`); n != 3 {
		t.Errorf("graph nodes: got %d, want 3", n)
	}
	if n := len(rowYs(t, body)); n != 1 {
		t.Errorf("linear rows: got %d distinct rows, want 1", n)
	}
}

// rowYs returns the distinct y coordinates of the graph node rects.
func rowYs(t *testing.T, body string) map[string]bool {
	t.Helper()
	ys := map[string]bool{}
	for _, m := range graphNodeY.FindAllStringSubmatch(body, -1) {
		ys[m[1]] = true
	}
	return ys
}

// The workflow page lists the runs of the workflow (phase 3, LIST): one
// row per execution — badge + first execution date + workflow ID, each row
// linking to its detail page, newest first. Live badges poll; terminal
// ones do not.
func TestWorkflowPageListsExecutions(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	newer := time.Date(2026, 5, 13, 13, 40, 0, 0, time.Local)
	older := time.Date(2026, 5, 13, 13, 30, 0, 0, time.Local)
	listExecutions := func(name string) ([]core.Execution, error) {
		return []core.Execution{
			{ID: "run-newer", State: core.StateSuccess, CreatedAt: newer},
			{ID: "run-older", State: core.StateQueued, CreatedAt: older},
		}, nil
	}
	srv := httptest.NewServer(New(fakeLister, okPaths, listExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows/wf-alpha")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`href="/executions/run-newer"`,
		`href="/executions/run-older"`,
		`class="badge success terminal"`,
		`class="badge queued"`,
		`hx-get="/status/run-older"`,
		`<span class="exec-date">13/05/2026 13:40</span>`,
		`<span class="exec-date">13/05/2026 13:30</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	if strings.Contains(body, `hx-get="/status/run-newer"`) {
		t.Error("terminal row badge still polls")
	}
	// Newest first: the newer run's row appears before the older run's
	iNewer, iOlder := strings.Index(body, "run-newer"), strings.Index(body, "run-older")
	if iNewer < 0 || iOlder < 0 || iNewer > iOlder {
		t.Errorf("rows are not newest first: newer at %d, older at %d", iNewer, iOlder)
	}
}

// The runs listing comes from the core: core down is a clean 503.
func TestWorkflowPageCoreDownIs503(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	downExecutions := func(name string) ([]core.Execution, error) {
		return nil, core.ErrNotLaunched
	}
	srv := httptest.NewServer(New(fakeLister, okPaths, downExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/workflows/wf-alpha"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// A page for a workflow that is not in the core registry is a clean 404.
func TestWorkflowPageUnknownIs404(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/workflows/ghost"); status != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", status, http.StatusNotFound)
	}
}
