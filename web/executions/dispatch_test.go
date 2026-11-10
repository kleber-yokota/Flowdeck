package executions

import (
	"io"
	"net/http"
	"os"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/web/internal/webtest"
)

// The dispatch (the POST half) against the booted core: the ACK is the
// execrow fragment — the badge (the id attribute, the poll target), the
// workflow id (the detail link), the elapsed and the cancel button — the
// run is a real run of the deterministic fixtures.

// The dispatch ACK (the execrow fragment): the badge queued (the id
// attribute), the workflow id (the detail link), the elapsed, the cancel
// button and no resume — no form in the ACK (the form lives on the
// workflow page).
func TestDispatchACKShowsRunlineRow(t *testing.T) {
	srv := bootedTestServer(t)

	ack := webtest.Ack(t, srv.URL, "noinput-test", nil)
	id := webtest.ExecutionID(t, ack)
	for _, want := range []string{
		`<div class="exec-row wf">`,
		`id="badge-` + id + `"`,
		`class="wf-id"`,
		`href="/executions/` + id + `"`,
		`<span class="elapsed">0s</span>`,
		`hx-post="/cancel/` + id + `"`,
	} {
		if !strings.Contains(ack, want) {
			t.Errorf("ACK does not contain %q: %s", want, ack)
			return
		}
	}
	if strings.Contains(ack, `class="resume"`) {
		t.Errorf("ACK has a resume button: %s", ack)
	}
	if strings.Contains(ack, `<form`) {
		t.Errorf("ACK contains a form: %s", ack)
	}
}

// A dispatch of a workflow that is not registered is a clean 404 (the
// registry is the source — no hardcoding).
func TestDispatchUnknownWorkflowIs404(t *testing.T) {
	srv := testServer(t, core.NewPort())

	resp, err := webtest.Post(t, srv.URL+"/run/ghost")
	if err != nil {
		t.Errorf("POST: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("dispatch: got %d want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// The dispatch ACK is dynamic: it must never be cached by the browser.
func TestDispatchNoStore(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := webtest.PostForm(t, srv.URL+"/run/noinput-test", nil)
	if err != nil {
		t.Errorf("POST: %v", err)
		return
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Errorf("read body: %v", err)
		return
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

// The dispatch of a not-launched core is a clean 503: the precondition is
// the source-registered graph with no boot (the per-test boots of the
// siblings tear the singleton down, so the test brings its own — the
// lookup of the spec is graph-dependent, and the 503 comes from the
// enqueue, not the lookup).
func TestDispatchCoreDownIs503(t *testing.T) {
	if err := boot.SetWorkflowsSource(os.DirFS("../../core/testworkflows")); err != nil {
		t.Errorf("set workflows source: %v", err)
		return
	}
	srv := testServer(t, core.NewPort())

	resp, err := webtest.Post(t, srv.URL+"/run/noinput-test")
	if err != nil {
		t.Errorf("POST: %v", err)
		return
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("dispatch: got %d want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}
