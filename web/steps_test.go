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

// The steps partial (phase 4, STEP 3, polling): a live run returns the
// executed-path SVG with the hx attributes that keep the 2s poll running.
func TestStepsPartialPollsWhenRunning(t *testing.T) {
	steps := func(id string) ([]core.Step, error) {
		return []core.Step{
			{Name: "fetch", State: status.Success},
			{Name: "parse", State: status.Success},
		}, nil
	}
	d := testPort()
	d.Status = stateStatuser(status.Running)
	d.Steps = steps
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-live/steps")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`<svg`,
		`hx-get="/executions/run-live/steps"`,
		`hx-trigger="every 2s"`,
		`>fetch<`,
		`>parse<`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment does not contain %q: %q", want, body)
		}
	}
}

// A terminal run stops the poll: the fragment has no hx attributes and
// shows the path that ran.
func TestStepsPartialTerminalStopsPolling(t *testing.T) {
	steps := func(id string) ([]core.Step, error) {
		return []core.Step{
			{Name: "fetch", State: status.Success},
			{Name: "store", State: status.Success},
		}, nil
	}
	d := testPort()
	d.Status = stateStatuser(status.Success)
	d.Steps = steps
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-done/steps")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{`<svg`, `>fetch<`, `>store<`} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment does not contain %q: %q", want, body)
		}
	}
	for _, no := range []string{`hx-trigger="every 2s"`, `hx-swap="outerHTML"`, `hx-target="this"`} {
		if strings.Contains(body, no) {
			t.Errorf("terminal fragment must not poll, but contains %q", no)
		}
	}
}

// A failed step is colored with the failed state (the node's class), the
// successful steps keep the success color.
func TestStepsPartialColorsTheFailedStep(t *testing.T) {
	steps := func(id string) ([]core.Step, error) {
		return []core.Step{
			{Name: "fetch", State: status.Success},
			{Name: "parse", State: status.Error},
		}, nil
	}
	d := testPort()
	d.Status = stateStatuser(status.Error)
	d.Steps = steps
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-failed/steps")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, `class="g-node success"`) || !strings.Contains(body, `>fetch<`) {
		t.Errorf("fragment must color the successful step: %q", body)
	}
	if !strings.Contains(body, `class="g-node error"`) || !strings.Contains(body, `>parse<`) {
		t.Errorf("fragment must color the failed step: %q", body)
	}
}

// The fragment keeps the steps in execution order (step_id asc).
func TestStepsPartialKeepsExecutionOrder(t *testing.T) {
	steps := func(id string) ([]core.Step, error) {
		return []core.Step{
			{Name: "fetch", State: status.Success},
			{Name: "parse", State: status.Success},
			{Name: "store", State: status.Success},
		}, nil
	}
	d := testPort()
	d.Status = stateStatuser(status.Success)
	d.Steps = steps
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-order/steps")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	fetch := strings.Index(body, ">fetch<")
	parse := strings.Index(body, ">parse<")
	store := strings.Index(body, ">store<")
	if fetch < 0 || parse < 0 || store < 0 {
		t.Fatalf("fragment missing steps: fetch=%d parse=%d store=%d", fetch, parse, store)
	}
	if !(fetch < parse && parse < store) {
		t.Errorf("steps out of execution order: fetch=%d parse=%d store=%d", fetch, parse, store)
	}
}

// A run the core does not know is a clean 404.
func TestStepsPartialUnknownExecutionIs404(t *testing.T) {
	statuser := func(id string) (status.State, error) { return "", core.ErrUnknownExecution }
	d := testPort()
	d.Status = statuser
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/executions/ghost/steps"); status != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", status, http.StatusNotFound)
	}
}

// A core that is down maps to a clean 503 (the poll keeps waiting).
func TestStepsPartialCoreDownIs503(t *testing.T) {
	statuser := func(id string) (status.State, error) { return "", core.ErrNotLaunched }
	d := testPort()
	d.Status = statuser
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/executions/abc/steps"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// The detail page embeds the executed path (phase 4, STEP 3): a live run
// polls the steps partial every 2s.
func TestExecutionDetailPagePollsExecutedPath(t *testing.T) {
	detailer := func(id string) (core.ExecutionDetail, error) {
		return core.ExecutionDetail{
			ID:        id,
			Name:      "wf-alpha",
			State:     status.Running,
			CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
			Attempts:  1,
		}, nil
	}
	d := testPort()
	d.Status = stateStatuser(status.Running)
	d.Detail = detailer
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-live")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`<svg`,
		`hx-get="/executions/run-live/steps"`,
		`class="g-node success"`,
		`>greet<`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// A terminal run on the detail page: the executed path is static (no
// hx attributes) and the badge stops polling too.
func TestExecutionDetailPageTerminalExecutedPath(t *testing.T) {
	detailer := func(id string) (core.ExecutionDetail, error) {
		return core.ExecutionDetail{
			ID:        id,
			Name:      "wf-alpha",
			State:     status.Success,
			CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
			Attempts:  1,
		}, nil
	}
	d := testPort()
	d.Status = stateStatuser(status.Success)
	d.Detail = detailer
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	status, body := get(t, srv.URL+"/executions/run-ok")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, `class="g-node success"`) || !strings.Contains(body, `>greet<`) {
		t.Errorf("page must show the executed path: %q", body)
	}
	// Terminal run: neither the badge nor the executed path polls.
	for _, no := range []string{`hx-get="/status/run-ok"`, `hx-get="/executions/run-ok/steps"`, `hx-trigger="every 2s"`} {
		if strings.Contains(body, no) {
			t.Errorf("terminal page must not poll, but contains %q", no)
		}
	}
}
