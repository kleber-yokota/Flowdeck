package catalog

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/web/internal/webtest"
)

// The workflow page (full page, opened from the wall) against the booted
// core: the dispatch button, the executions list (newest first) and the
// generated paths graph — no stubs, the rows are real runs.

// The no-input workflow page: the plain Run button (no form) and the
// executions list that receives the ACK rows.
func TestWorkflowPageRendersDispatchButtonAndList(t *testing.T) {
	srv := bootedTestServer(t)

	status, body := webtest.Get(t, srv.URL+"/workflows/noinput-test")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`hx-post="/run/noinput-test"`,
		`id="executions"`,
		"Run",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	if strings.Contains(body, `<form`) {
		t.Error("no-input workflow page has a form")
	}
}

// The workflow page layout: the executions list (what was executed) is on
// the left and the generated paths graph is on the right.
func TestWorkflowPageColumnsLayout(t *testing.T) {
	srv := bootedTestServer(t)

	status, body := webtest.Get(t, srv.URL+"/workflows/linear-test")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	iExec, iGraph := strings.Index(body, `id="executions"`), strings.Index(body, `<div class="graph">`)
	if iExec < 0 || iGraph < 0 || iExec > iGraph {
		t.Errorf("executions must come before the graph: executions at %d, graph at %d", iExec, iGraph)
	}
}

// The workflow page lists the runs of the workflow: one
// row per execution — badge + first execution date + workflow ID, each row
// linking to its detail page, newest first. Terminal badges do not poll.
// (The runs are enqueued straight on the core — the dispatch route is the
// executions package's, the listing only needs runs.)
func TestWorkflowPageListsExecutions(t *testing.T) {
	srv := bootedTestServer(t)

	enqueue := func() string {
		h, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "noinput-test", nil)
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		return h.GetWorkflowID()
	}
	older := enqueue()
	// The ordering assertion reads the created_at (millisecond precision):
	// the gap keeps the two rows in distinct created_at.
	time.Sleep(10 * time.Millisecond)
	newer := enqueue()
	for _, id := range []string{older, newer} {
		deadline := time.Now().Add(30 * time.Second)
		for {
			st, err := reads.Status(id)
			if err != nil {
				t.Fatalf("status: %v", err)
			}
			if st == status.Success {
				break
			}
			if st != status.Queued && st != status.Running {
				t.Fatalf("run %s: state %s", id, st)
			}
			if time.Now().After(deadline) {
				t.Fatalf("run %s: no success: %s", id, st)
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	status, body := webtest.Get(t, srv.URL+"/workflows/noinput-test")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, id := range []string{older, newer} {
		for _, want := range []string{
			`href="/executions/` + id + `"`,
			`class="badge success terminal"`,
			`<span class="exec-date">`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("page does not contain %q", want)
			}
		}
		// Terminal badge: no poll on the listing row.
		if strings.Contains(body, `hx-get="/status/`+id+`"`) {
			t.Errorf("terminal row badge still polls: %s", body)
		}
	}
	// Newest first: the newer run's row appears before the older run's.
	iOlder, iNewer := strings.Index(body, older), strings.Index(body, newer)
	if iOlder < 0 || iNewer < 0 {
		t.Fatalf("rows missing: older=%d newer=%d", iOlder, iNewer)
	}
	if !(iNewer < iOlder) {
		t.Errorf("rows are not newest first: newer at %d, older at %d", iNewer, iOlder)
	}
}

// A page for a workflow that is not registered is a clean 404 (the
// registry is the source — the graph and the booted registry both agree).
func TestWorkflowPageUnknownIs404(t *testing.T) {
	srv := testServer(t, core.NewPort())

	if status, _ := webtest.Get(t, srv.URL+"/workflows/ghost"); status != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", status, http.StatusNotFound)
	}
}
