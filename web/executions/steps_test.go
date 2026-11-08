package executions

import (
	"net/http"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/web/internal/webtest"
)

// The executed-path partial (the steps of the run) against real runs
// (the booted core, no stubs): the live fragment keeps the 2s poll, the
// terminal fragment stops it and keeps the steps in execution order.
// (The SVG coloring/ordering of the graph itself is unit-covered in
// web/graph — here the HTTP fragment is what the page swaps.)

// A live run (the block-test run): the fragment carries the hx attributes
// that keep the 2s poll running.
func TestStepsPartialPollsWhenLive(t *testing.T) {
	srv := bootedTestServer(t)

	id := blockRun(t, srv)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Fatalf("steps: %d", code)
	}
	for _, want := range []string{
		`hx-get="/executions/` + id + `/steps`,
		`hx-trigger="every 2s"`,
		`hx-swap="outerHTML"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment does not contain %q: %s", want, body)
		}
	}
}

// A terminal run (the live-test success run): the fragment has no hx
// attributes (the poll stops) and shows the step that ran.
func TestStepsPartialTerminalStopsPolling(t *testing.T) {
	srv := bootedTestServer(t)

	id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "live-test", nil))
	webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Fatalf("steps: %d", code)
	}
	if !strings.Contains(body, ">live<") {
		t.Errorf("fragment must show the executed step: %s", body)
	}
	for _, no := range []string{`hx-trigger="every 2s"`, `hx-swap="outerHTML"`, `hx-target="this"`} {
		if strings.Contains(body, no) {
			t.Errorf("terminal fragment must not poll, but contains %q: %s", no, body)
		}
	}
}

// The fragment keeps the steps in execution order (the linear-test run:
// fetch → parse → transform → store).
func TestStepsPartialKeepsExecutionOrder(t *testing.T) {
	srv := bootedTestServer(t)

	id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "linear-test", nil))
	webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Fatalf("steps: %d", code)
	}
	fetch := strings.Index(body, ">fetch<")
	parse := strings.Index(body, ">parse<")
	transform := strings.Index(body, ">transform<")
	store := strings.Index(body, ">store<")
	if fetch < 0 || parse < 0 || transform < 0 || store < 0 {
		t.Fatalf("fragment missing steps (fetch=%d parse=%d transform=%d store=%d): %s", fetch, parse, transform, store, body)
	}
	if !(fetch < parse && parse < transform && transform < store) {
		t.Errorf("steps out of execution order (fetch=%d parse=%d transform=%d store=%d)", fetch, parse, transform, store)
	}
}

// The steps of an execution the core does not know is a clean 404 (the
// poll keeps waiting).
func TestStepsPartialUnknownExecutionIs404(t *testing.T) {
	srv := bootedTestServer(t)

	if code, _ := webtest.Get(t, srv.URL+"/executions/00000000-0000-0000-0000-000000000000/steps"); code != http.StatusNotFound {
		t.Fatalf("steps: got %d want %d", code, http.StatusNotFound)
	}
}

// The poll of a not-launched core is a clean 503 (the poll keeps waiting).
func TestStepsPartialCoreDownIs503(t *testing.T) {
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/executions/abc/steps"); code != http.StatusServiceUnavailable {
		t.Fatalf("steps: got %d want %d", code, http.StatusServiceUnavailable)
	}
}
