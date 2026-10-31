package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/graph"
	"flowdeck/core/status"
)

// The workflow page (full page, opened from the list) has the dispatch
// button and the executions list that receives the ACK rows.
func TestWorkflowPageRendersDispatchButtonAndList(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	d := testPort()
	d.List = fakeLister
	srv := httptest.NewServer(New(d, okReady))
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
	// The empty graph returns no paths: no graph nodes are rendered.
	if strings.Contains(body, `class="g-node"`) {
		t.Error("graph nodes rendered without paths")
	}
}

// The workflow page layout (phase 4): the executions list (what was
// executed) is on the left and the generated paths graph is on the right.
func TestWorkflowPageColumnsLayout(t *testing.T) {
	d := testPort()
	d.List = func() []core.Workflow { return []core.Workflow{{Name: "wf-alpha"}} }
	d.Graph = fakeGraph("wf-alpha", [][]string{{"one", "two"}})
	srv := httptest.NewServer(New(d, okReady))
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
	wg := &graph.WorkflowGraph{
		Root: &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence, Children: []graph.Child{
			{Block: &graph.Block{ID: "b1", Level: 0, Type: graph.BlockIf,
				Children: []graph.Child{{Node: &graph.Node{Name: "left", Kind: graph.NodeStep}}},
				Else:     &graph.Block{ID: "b2", Level: 0, Children: []graph.Child{{Node: &graph.Node{Name: "right", Kind: graph.NodeStep}}}},
			}},
		}},
	}
	d := testPort()
	d.List = func() []core.Workflow { return []core.Workflow{{Name: "wf-alpha"}} }
	d.Graph = &graph.PathGraph{ByName: map[string]*graph.WorkflowGraph{"wf-alpha": wg}}
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows/wf-alpha")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	// The if forks: both branch boxes are drawn, and the L0 outline is
	// present (the root sequence).
	for _, want := range []string{
		`<svg`,
		">left<",
		">right<",
		`class="g-block l0"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// A workflow without an if renders a single linear row (all the nodes on
// the same row).
func TestWorkflowPageRendersLinearGraph(t *testing.T) {
	d := testPort()
	d.List = func() []core.Workflow { return []core.Workflow{{Name: "wf-alpha"}} }
	d.Graph = fakeGraph("wf-alpha", [][]string{{"fetch", "parse", "store"}})
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows/wf-alpha")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if n := strings.Count(body, `class="g-node"`); n != 3 {
		t.Errorf("graph nodes: got %d, want 3", n)
	}
	for _, s := range []string{`>fetch</text>`, `>parse</text>`, `>store</text>`} {
		if !strings.Contains(body, s) {
			t.Errorf("page does not contain %q", s)
		}
	}
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
			{ID: "run-newer", State: status.Success, CreatedAt: newer},
			{ID: "run-older", State: status.Queued, CreatedAt: older},
		}, nil
	}
	d := testPort()
	d.List = fakeLister
	d.Executions = listExecutions
	srv := httptest.NewServer(New(d, okReady))
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
	d := testPort()
	d.List = fakeLister
	d.Executions = downExecutions
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/workflows/wf-alpha"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// A page for a workflow that is not in the core registry is a clean 404.
func TestWorkflowPageUnknownIs404(t *testing.T) {
	srv := httptest.NewServer(New(testPort(), okReady))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/workflows/ghost"); status != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", status, http.StatusNotFound)
	}
}
