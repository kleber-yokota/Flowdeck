package web

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/core/graph"
)

// POST /run is the dispatch (phase 2): it enqueues in the core and returns
// the execution row fragment: "queued" badge + the visible workflow ID
// (the ACK) — the browser prepends it to the executions list.
func TestRunRendersQueuedBadgeWithWorkflowID(t *testing.T) {
	srv := httptest.NewServer(New(testPort(), okReady))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/run/wf-alpha", "application/octet-stream", nil)
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	for _, want := range []string{
		`<li class="execution">`,
		`data-workflow-id="00000000-0000-0000-0000-000000000001"`,
		"Queued",
		`<a class="wf-id" href="/executions/00000000-0000-0000-0000-000000000001">00000000-0000-0000-0000-000000000001</a>`,
		// The badge starts the 2s poll right after the dispatch; the whole
		// badge is replaced (outerHTML) on each poll, so the old state never
		// wraps the new one.
		`hx-get="/status/00000000-0000-0000-0000-000000000001"`,
		`hx-trigger="every 2s"`,
		`hx-swap="outerHTML"`,
		// The ACK row already carries the first execution date
		`<span class="exec-date">13/05/2026 13:35</span>`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("fragment does not contain %q: %q", want, body)
		}
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

// Phase 5, EXECUTE: the web sanitizes the values against the spec before
// the enqueue — a field that is not in the spec is a clean 400.
func TestRunUnknownFieldIs400(t *testing.T) {
	d := testPort()
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	resp, err := postForm(t, srv.URL+"/run/wf-alpha", url.Values{"msg": {"x"}, "bogus": {"y"}})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// Phase 5, EXECUTE: a value that does not convert to the input's type is a
// clean 400 (the spec says int, the form posts "x").
func TestRunBadIntIs400(t *testing.T) {
	d := testPort()
	d.Spec = func(name string) (graph.Spec, error) {
		return graph.Spec{Inputs: []graph.Param{{Name: "n", Type: "int"}}}, nil
	}
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	resp, err := postForm(t, srv.URL+"/run/wf-alpha", url.Values{"n": {"x"}})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusBadRequest)
	}
}

// Phase 5, EXECUTE: the web forwards the clean values to the core (the
// sanitized input, not the raw form map) — the ACK row is rendered.
func TestRunForwardsCleanValues(t *testing.T) {
	var payload any
	d := testPort()
	d.Enqueue = func(name string, p any) (core.Execution, error) {
		payload = p
		return okEnqueuer(name, p)
	}
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	resp, err := postForm(t, srv.URL+"/run/wf-alpha", url.Values{"msg": {"left"}})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if payload != "left" {
		t.Fatalf("payload: got %v, want the clean value \"left\"", payload)
	}
}

// A workflow that is not in the core registry is a clean 404.
func TestRunUnknownWorkflowIs404(t *testing.T) {
	enqueue := func(name string, payload any) (core.Execution, error) {
		return core.Execution{}, fmt.Errorf("%w: %s", core.ErrUnknownWorkflow, name)
	}
	d := testPort()
	d.Enqueue = enqueue
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/run/ghost", "application/octet-stream", nil)
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// A core failure that is not "unknown workflow" is a clean 500.
func TestRunEnqueueFailureIs500(t *testing.T) {
	enqueue := func(name string, payload any) (core.Execution, error) {
		return core.Execution{}, errors.New("boom")
	}
	d := testPort()
	d.Enqueue = enqueue
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/run/wf-alpha", "application/octet-stream", nil)
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusInternalServerError)
	}
}
