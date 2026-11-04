package executions

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/status"
	"flowdeck/web/internal/webtest"
)

// The web cancels a run from the execution page (the cancel sequence): the
// live run (queued/running) offers the Cancel button that POSTs to
// /cancel/{id} and targets #badge; the ACK renders the cancelled badge
// (terminal — the 2s poll stops). Real runs (the booted core, no stubs).

// The booted cancel: the dispatched block-test run is live (the block
// step blocks until the cancel — the deterministic Running state) — the
// detail page offers the Cancel button (POST /cancel/{id}, targeting the
// badge wrapper); the cancel ACKs the cancelled badge (terminal), and the
// detail page then renders the static cancelled badge (no 2s poll — the
// poll stops).
func TestCancelBootedRunRendersCancelledBadge(t *testing.T) {
	srv := bootedTestServer(t)

	id := blockRun(t, srv)

	// The run is live (queued/running): the detail page offers the Cancel
	// button.
	deadline := time.Now().Add(30 * time.Second)
	for {
		code, body := webtest.Get(t, srv.URL+"/executions/"+id)
		if code == http.StatusOK && strings.Contains(body, `hx-post="/cancel/`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no cancel button: %d, %s", code, body)
		}
		time.Sleep(200 * time.Millisecond)
	}

	c, err := webtest.Post(t, srv.URL+"/cancel/"+id)
	if err != nil {
		t.Fatalf("POST /cancel: %v", err)
	}
	ackBody := webtest.ReadAll(t, c)
	if c.StatusCode != http.StatusOK {
		t.Fatalf("cancel: %d, %s", c.StatusCode, ackBody)
	}
	if !strings.Contains(ackBody, `class="badge cancelled terminal"`) {
		t.Fatalf("ACK is not the cancelled badge: %s", ackBody)
	}

	// The detail page keeps the static cancelled badge (no 2s poll).
	code, body := webtest.Get(t, srv.URL+"/executions/"+id)
	if code != http.StatusOK {
		t.Fatalf("detail: %d", code)
	}
	if !strings.Contains(body, `class="badge cancelled terminal"`) {
		t.Fatalf("detail without the cancelled badge: %s", body)
	}
	if strings.Contains(body, `hx-get="/status/`) {
		t.Fatalf("cancelled detail still polls the badge: %s", body)
	}
}

// The detail pages of the terminal runs (success, error, cancelled) offer
// no Cancel button; the cancelled page keeps the static cancelled badge.
func TestExecutionDetailNoCancelOnTerminal(t *testing.T) {
	for _, st := range []status.State{status.Success, status.Error, status.Cancelled} {
		t.Run(string(st), func(t *testing.T) {
			srv := bootedTestServer(t)

			id := runInState(t, srv, st)
			code, body := webtest.Get(t, srv.URL+"/executions/"+id)
			if code != http.StatusOK {
				t.Fatalf("detail: %d", code)
			}
			if strings.Contains(body, `hx-post="/cancel/`) {
				t.Fatalf("terminal page offers a cancel: %s", body)
			}
			if st == status.Cancelled && !strings.Contains(body, `class="badge cancelled terminal"`) {
				t.Fatalf("cancelled page without the static badge: %s", body)
			}
		})
	}
}

// The cancel of an id that is not known by the core is a clean 404 (the
// core maps it to ErrUnknownExecution).
func TestCancelUnknownIs404(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := webtest.Post(t, srv.URL+"/cancel/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("POST /cancel: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cancel unknown: got %d, want 404", resp.StatusCode)
	}
}

// The cancel of a run that is not live (here: a success run) is a clean
// 400 (the core maps it to ErrNotCancellable).
func TestCancelOfNotLiveIs400(t *testing.T) {
	srv := bootedTestServer(t)

	id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "live-test", nil))
	webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)

	resp, err := webtest.Post(t, srv.URL+"/cancel/"+id)
	if err != nil {
		t.Fatalf("POST /cancel: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cancel of settled: got %d, want 400", resp.StatusCode)
	}
}

// A not-launched core maps to a clean 503 (the real port, no stubs).
func TestCancelCoreDownIs503(t *testing.T) {
	skipBooted(t)
	srv := testServer(t, core.NewPort())

	resp, err := webtest.Post(t, srv.URL+"/cancel/abc")
	if err != nil {
		t.Fatalf("POST /cancel: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("cancel core down: got %d, want 503", resp.StatusCode)
	}
}
