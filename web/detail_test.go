package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
)

// The detail page (phase 3, DETAIL) renders the badge, the retry rows
// (retries 1..N-1 are recovered failures; the final attempt is the badge)
// and, on failure, the error and the failed step.
func TestExecutionDetailRendersFailure(t *testing.T) {
	detailer := func(id string) (core.ExecutionDetail, error) {
		return core.ExecutionDetail{
			ID:         id,
			Name:       "wf-alpha",
			State:      core.StateError,
			CreatedAt:  time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
			Attempts:   3,
			Error:      "boom: step failed",
			FailedStep: "boom-step",
		}, nil
	}
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, detailer, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-failed")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`class="badge error terminal"`,
		"first execution: 13/05/2026 13:35",
		// retries = attempts − 1: 3 attempts → 2 retries; the final failure
		// is the badge + error section.
		"retries: 2",
		"retry 1 → failed (recovered)",
		"retry 2 → failed (recovered)",
		"boom: step failed",
		"failed step: <code>boom-step</code>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// No row for the final attempt — the badge carries it.
	if strings.Contains(body, "retry 3") {
		t.Errorf("final attempt must not be a retry row: %q", body)
	}
	// Terminal badge: the poll is over, no hx attributes.
	for _, no := range []string{`hx-get="/status/run-failed"`, `hx-trigger="every 2s"`} {
		if strings.Contains(body, no) {
			t.Errorf("terminal page must not poll, but contains %q", no)
		}
	}
}

// A live run on the detail page keeps polling the badge every 2s.
func TestExecutionDetailBadgePollsWhenRunning(t *testing.T) {
	detailer := func(id string) (core.ExecutionDetail, error) {
		return core.ExecutionDetail{ID: id, Name: "wf-alpha", State: core.StateRunning, Attempts: 1,
			CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local)}, nil
	}
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, detailer, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-live")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`hx-get="/status/run-live"`,
		`hx-trigger="every 2s"`,
		"retries: 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// No retry yet: no "retry" row at all.
	if strings.Contains(body, "retry ") {
		t.Errorf("live run without retries must not list attempts: %q", body)
	}
}

// A clean run (no error) shows zero retries: the counter is attempts − 1,
// so a success on the first attempt has none.
func TestExecutionDetailSuccessHasZeroRetries(t *testing.T) {
	detailer := func(id string) (core.ExecutionDetail, error) {
		return core.ExecutionDetail{
			ID:        id,
			Name:      "wf-alpha",
			State:     core.StateSuccess,
			CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
			Attempts:  1,
		}, nil
	}
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, detailer, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-ok")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`class="badge success terminal"`,
		"first execution: 13/05/2026 13:35",
		"retries: 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// No error, no retry: no "retry" row at all.
	if strings.Contains(body, "retry ") {
		t.Errorf("clean run must not list attempts: %q", body)
	}
	// No failure section on a success.
	if strings.Contains(body, "failed step") {
		t.Errorf("success page must not show a failed step: %q", body)
	}
}

// A run the core does not know is a clean 404.
func TestExecutionDetailUnknownIs404(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/executions/ghost"); status != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", status, http.StatusNotFound)
	}
}

// A core that is down maps to a clean 503.
func TestExecutionDetailCoreDownIs503(t *testing.T) {
	downDetail := func(id string) (core.ExecutionDetail, error) {
		return core.ExecutionDetail{}, core.ErrNotLaunched
	}
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, downDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/executions/abc"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}
