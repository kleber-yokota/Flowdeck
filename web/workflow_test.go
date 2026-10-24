package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
)

// The workflow page (full page, opened from the list) has the dispatch
// button and the executions list that receives the ACK rows.
func TestWorkflowPageRendersDispatchButtonAndList(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	srv := httptest.NewServer(New(fakeLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
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
	srv := httptest.NewServer(New(fakeLister, listExecutions, okEnqueuer, okStatuser, okDetail, okPing))
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
	srv := httptest.NewServer(New(fakeLister, downExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/workflows/wf-alpha"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// A page for a workflow that is not in the core registry is a clean 404.
func TestWorkflowPageUnknownIs404(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/workflows/ghost"); status != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", status, http.StatusNotFound)
	}
}
