package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/status"
)

// The web cancels a run from the execution page (the cancel sequence): the
// live run (queued/running) offers the Cancel button that POSTs to
// /cancel/{id} and targets #badge; the ACK renders the cancelled badge
// (terminal — the 2s poll stops).

// The detail page of a live run (running) offers the Cancel button
// (POST /cancel/{id}, targeting the badge wrapper).
func TestExecutionDetailCancelOnLiveRun(t *testing.T) {
	detailer := func(id string) (core.DetailPath, error) {
		return core.DetailPath{
			ExecutionDetail: core.ExecutionDetail{
				ID:        id,
				Name:      "wf-alpha",
				State:     status.Running,
				CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
				Attempts:  1,
			},
			RunPath: core.RunPath{State: status.Running},
		}, nil
	}
	d := testPort()
	d.Detail = detailer
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	code, body := get(t, srv.URL+"/executions/run-live")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	for _, want := range []string{
		`hx-post="/cancel/run-live"`,
		`hx-target="#badge"`,
		"Cancel",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// The detail pages of the terminal runs (success, error, cancelled) offer
// no Cancel button; the cancelled page keeps the static cancelled badge.
func TestExecutionDetailNoCancelOnTerminal(t *testing.T) {
	for _, st := range []status.State{status.Success, status.Error, status.Cancelled} {
		t.Run(string(st), func(t *testing.T) {
			detailer := func(id string) (core.DetailPath, error) {
				return core.DetailPath{
					ExecutionDetail: core.ExecutionDetail{
						ID:        id,
						Name:      "wf-alpha",
						State:     st,
						CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
						Attempts:  1,
					},
					RunPath: core.RunPath{State: st},
				}, nil
			}
			d := testPort()
			d.Detail = detailer
			srv := httptest.NewServer(New(d, okReady))
			defer srv.Close()

			code, body := get(t, srv.URL+"/executions/run-terminal")
			if code != http.StatusOK {
				t.Fatalf("status: got %d want %d", code, http.StatusOK)
			}
			if strings.Contains(body, "/cancel/") {
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
	canceller := func(id string) (core.Execution, error) {
		return core.Execution{}, core.ErrUnknownExecution
	}
	d := testPort()
	d.Cancel = canceller
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	resp, err := post(t, srv.URL+"/cancel/ghost")
	if err != nil {
		t.Fatalf("POST /cancel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("cancel unknown: got %d, want 404", resp.StatusCode)
	}
}

// The cancel of a run that is not live is a clean 400 (the core maps it to
// ErrNotCancellable).
func TestCancelOfNotLiveIs400(t *testing.T) {
	canceller := func(id string) (core.Execution, error) {
		return core.Execution{}, core.ErrNotCancellable
	}
	d := testPort()
	d.Cancel = canceller
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	resp, err := post(t, srv.URL+"/cancel/run-settled")
	if err != nil {
		t.Fatalf("POST /cancel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("cancel of settled: got %d, want 400", resp.StatusCode)
	}
}

// A core that is down maps to a clean 503.
func TestCancelCoreDownIs503(t *testing.T) {
	canceller := func(id string) (core.Execution, error) {
		return core.Execution{}, core.ErrNotLaunched
	}
	d := testPort()
	d.Cancel = canceller
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	resp, err := post(t, srv.URL+"/cancel/run-live")
	if err != nil {
		t.Fatalf("POST /cancel: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("cancel core down: got %d, want 503", resp.StatusCode)
	}
}

// The booted twin (no stubs): the dispatched pause run is live — the
// detail page offers the Cancel button; the cancel ACKs the cancelled
// badge (terminal), and the detail page then renders the static cancelled
// badge (no 2s poll — the poll stops).
func TestCancelBootedRunRendersCancelledBadge(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := post(t, srv.URL+"/run/pause")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := readAll(t, resp)
	if resp.StatusCode != http.StatusOK || !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("dispatch pause: %d, %s", resp.StatusCode, ack)
	}
	id := executionID(t, ack)

	// The run is live (queued/running): the detail page offers the Cancel
	// button.
	deadline := time.Now().Add(30 * time.Second)
	for {
		code, body := get(t, srv.URL+"/executions/"+id)
		if code == http.StatusOK && strings.Contains(body, `hx-post="/cancel/`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no cancel button: %d, %s", code, body)
		}
		time.Sleep(200 * time.Millisecond)
	}

	c, err := post(t, srv.URL+"/cancel/"+id)
	if err != nil {
		t.Fatalf("POST /cancel: %v", err)
	}
	ackBody := readAll(t, c)
	c.Body.Close()
	if c.StatusCode != http.StatusOK {
		t.Fatalf("cancel: %d, %s", c.StatusCode, ackBody)
	}
	if !strings.Contains(ackBody, `class="badge cancelled terminal"`) {
		t.Fatalf("ACK is not the cancelled badge: %s", ackBody)
	}

	// The detail page keeps the static cancelled badge (no 2s poll).
	code, body := get(t, srv.URL+"/executions/"+id)
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
