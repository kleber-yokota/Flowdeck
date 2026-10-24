package web

import (
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
)

func emptyLister() []core.Workflow { return nil }

func okExecutions(name string) ([]core.Execution, error) { return nil, nil }

func okPing() error { return nil }

// The ACK execution of okEnqueuer: id + queued state + created_at.
func okEnqueuer(name string, payload any) (core.Execution, error) {
	return core.Execution{
		ID:        "00000000-0000-0000-0000-000000000001",
		State:     core.StateQueued,
		CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
	}, nil
}

func okStatuser(workflowID string) (core.State, error) {
	return core.StateQueued, nil
}

func okDetail(id string) (core.ExecutionDetail, error) {
	return core.ExecutionDetail{}, core.ErrUnknownExecution
}

// stateStatuser builds a statuser stub that always returns the given state.
func stateStatuser(state core.State) func(string) (core.State, error) {
	return func(workflowID string) (core.State, error) { return state, nil }
}

func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

func TestRootRendersDashboardWithHTMX(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	// The HTMX shell: the vendored script tag and the HTMX trigger that
	// loads the partial from the core registry.
	for _, want := range []string{
		`hx-get="/workflows"`,
		`<script src="/static/htmx.min.js" defer></script>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// No hardcoding: the partial lists exactly what the lister (the core) returns,
// and the base page knows nothing about specific workflows.
func TestWorkflowsPartialListsCoreRegistry(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{
			{Name: "wf-alpha"},
			{Name: "wf-beta"},
		}
	}
	srv := httptest.NewServer(New(fakeLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{"wf-alpha", "wf-beta"} {
		if !strings.Contains(body, want) {
			t.Errorf("partial does not contain %q", want)
		}
	}

	// The full page does not hardcode workflow names
	_, page := get(t, srv.URL+"/")
	for _, hard := range []string{"wf-alpha", "wf-beta"} {
		if strings.Contains(page, hard) {
			t.Errorf("page hardcodes %q", hard)
		}
	}
}

func TestWorkflowsPartialEmptyRegistry(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, "No workflows registered") {
		t.Errorf("empty partial does not contain the message: %q", body)
	}
}

// The fragment is dynamic: it must never be cached by the browser.
func TestWorkflowsPartialNoStore(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/workflows")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

func TestStaticServesVendoredHTMX(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/static/htmx.min.js")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	// The vendored bundle declares its own version
	if !strings.Contains(body, `version:"2.0.4"`) {
		t.Error("vendored htmx does not declare version 2.0.4")
	}
}

// The health endpoint mirrors the core: 200 when the core is up,
// 503 when it is not.
func TestHealthz(t *testing.T) {
	up := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer up.Close()
	if status, _ := get(t, up.URL+"/healthz"); status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}

	down := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, func() error { return core.ErrNotLaunched }))
	defer down.Close()
	if status, _ := get(t, down.URL+"/healthz"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// POST /run is the dispatch (phase 2): it enqueues in the core and returns
// the execution row fragment: "queued" badge + the visible workflow ID
// (the ACK) — the browser prepends it to the executions list.
func TestRunRendersQueuedBadgeWithWorkflowID(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
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

// A workflow that is not in the core registry is a clean 404.
func TestRunUnknownWorkflowIs404(t *testing.T) {
	enqueue := func(name string, payload any) (core.Execution, error) {
		return core.Execution{}, fmt.Errorf("%w: %s", core.ErrUnknownWorkflow, name)
	}
	srv := httptest.NewServer(New(emptyLister, okExecutions, enqueue, okStatuser, okDetail, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okExecutions, enqueue, okStatuser, okDetail, okPing))
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

// The partial links each registered workflow to its page (zero hardcoding
// in the base page): the dispatch button lives on that page.
func TestWorkflowsPartialLinksToWorkflowPages(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	srv := httptest.NewServer(New(fakeLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, `href="/workflows/wf-alpha"`) {
		t.Errorf("partial does not link to the workflow page: %q", body)
	}
	_, page := get(t, srv.URL+"/")
	if strings.Contains(page, `href="/workflows/`) {
		t.Error("page hardcodes workflow links")
	}
}

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
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, detailer, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, detailer, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, detailer, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, downDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/executions/abc"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// A render error must yield a clean 500 (nothing partial in flight), not a
// truncated 200 fragment injected into the HTMX DOM.
func TestRenderErrorIsClean500(t *testing.T) {
	type noName struct{ Other string }
	broken := template.Must(template.New("boom").Parse("{{ .Name }}"))

	rec := httptest.NewRecorder()
	render(broken, rec, "boom", noName{}) // .Name on a struct without it → Execute fails

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "error rendering") {
		t.Fatalf("body: %q", rec.Body.String())
	}
}

// GET /status is the poll half of the dispatch (phase 2): it returns the
// badge fragment for each of the four states, and only the non-terminal
// badges carry the hx attributes that keep the poll running.
func TestStatusBadgeRendersState(t *testing.T) {
	for state, want := range map[core.State]string{
		core.StateQueued:  `class="badge queued"`,
		core.StateRunning: `class="badge running"`,
		core.StateSuccess: `class="badge success terminal"`,
		core.StateError:   `class="badge error terminal"`,
	} {
		t.Run(string(state), func(t *testing.T) {
			srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, stateStatuser(state), okDetail, okPing))
			defer srv.Close()

			status, body := get(t, srv.URL+"/status/abc")
			if status != http.StatusOK {
				t.Fatalf("status: got %d want %d", status, http.StatusOK)
			}
			if !strings.Contains(body, want) {
				t.Errorf("fragment does not contain %q: %q", want, body)
			}
			// Live badges poll and are swapped whole (outerHTML) — the old
			// badge element never wraps the new one. Terminal badges drop
			// both attributes and stop the poll.
			polling := state == core.StateQueued || state == core.StateRunning
			for attr, wantAttr := range map[string]bool{
				`hx-trigger="every 2s"`: polling,
				`hx-swap="outerHTML"`:   polling,
			} {
				if has := strings.Contains(body, attr); has != wantAttr {
					t.Errorf("%s present: got %v want %v: %q", attr, has, wantAttr, body)
				}
			}
		})
	}
}

// The poll response must be exactly one complete badge element, not a bare
// text fragment: htmx swaps it in with outerHTML, which replaces the old
// badge element entirely. If the fragment were not a full element (or the
// swap fell back to innerHTML), the new badge would be nested inside the
// old one and the old state's outline would stay visible — the "silhouette"
// bug.
func TestStatusBadgeFragmentIsSingleCompleteElement(t *testing.T) {
	for state := range map[core.State]bool{
		core.StateQueued:  true,
		core.StateRunning: true,
		core.StateSuccess: true,
		core.StateError:   true,
	} {
		t.Run(string(state), func(t *testing.T) {
			srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, stateStatuser(state), okDetail, okPing))
			defer srv.Close()

			status, body := get(t, srv.URL+"/status/abc")
			if status != http.StatusOK {
				t.Fatalf("status: got %d want %d", status, http.StatusOK)
			}
			fragment := strings.TrimSpace(body)
			if !strings.HasPrefix(fragment, `<span class="badge`) || !strings.HasSuffix(fragment, `</span>`) {
				t.Fatalf("fragment is not a single span root: %q", body)
			}
			// Exactly one span root: no nested badge, no leftover wrapper.
			if spans := len(strings.Split(fragment, "<span")) - 1; spans != 1 {
				t.Errorf("fragment contains %d <span> elements, want 1: %q", spans, body)
			}
		})
	}
}

// An ID that is not known by the core is a clean 404.
func TestStatusUnknownExecutionIs404(t *testing.T) {
	statuser := func(workflowID string) (core.State, error) {
		return "", core.ErrUnknownExecution
	}
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, statuser, okDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/status/ghost"); status != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", status, http.StatusNotFound)
	}
}

// A core that is down maps to a clean 503 (the poll keeps waiting).
func TestStatusCoreDownIs503(t *testing.T) {
	statuser := func(workflowID string) (core.State, error) {
		return "", core.ErrNotLaunched
	}
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, statuser, okDetail, okPing))
	defer srv.Close()

	if status, _ := get(t, srv.URL+"/status/abc"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// The poll fragment is dynamic: it must never be cached by the browser.
func TestStatusBadgeNoStore(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, stateStatuser(core.StateQueued), okDetail, okPing))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/status/abc")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/does-not-exist")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPostRootReturns405(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okExecutions, okEnqueuer, okStatuser, okDetail, okPing))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/", "application/octet-stream", nil)
	if err != nil {
		t.Fatalf("POST /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
