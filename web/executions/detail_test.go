package executions

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/web/internal/webtest"
)

// The execution page (v6.3) renders real runs (the booted core, no stubs):
// the badge and the split fragment (the meta row, the actions, the step
// log with the child blocks and the dashed tail, and the sticky executed
// path) — no failure section (the state is on the badge, the failed step
// is highlighted in the log).

// The detail page of a live run (running — the block-test run) offers the
// Cancel button and polls (the badge and the split are live); the retries
// count is 0 (nothing failed yet).
func TestExecutionDetailLivePollsBadgeAndSteps(t *testing.T) {
	srv := bootedTestServer(t)

	id := blockRun(t, srv)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id)
	if code != http.StatusOK {
		t.Errorf("detail: %d", code)
	}
	for _, want := range []string{
		`hx-get="/status/` + id + `"`,
		`hx-trigger="every 2s"`,
		"retries: 0",
		`hx-post="/cancel/` + id + `"`,
		`hx-get="/executions/` + id + `/steps`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q: %s", want, body)
		}
	}
	if strings.Contains(body, "retry 1") {
		t.Errorf("live run without retries must not list attempts: %s", body)
	}
}

// The detail page of a success run (live-test, the default mode) is
// terminal: the static success badge (no 2s poll — the poll stops), the
// split is static and shows the step that ran, and there is no retry row,
// no cancel and no resume.
func TestExecutionDetailSuccessIsTerminal(t *testing.T) {
	srv := bootedTestServer(t)

	id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "live-test", nil))
	body := webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)
	for _, want := range []string{
		`class="badge success terminal"`,
		"retries: 0",
		">live<",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q: %s", want, body)
		}
	}
	for _, no := range []string{
		`hx-get="/status/`,
		`hx-get="/executions/" + id + "/steps"`,
		`hx-post="/cancel/`,
		`hx-post="/resume/`,
		"retry 1",
		"failed step",
		"children-section",
	} {
		if strings.Contains(body, no) {
			t.Errorf("success page must not contain %q: %s", no, body)
		}
	}
}

// The meta row of a success run: a live-test dispatch with the mode input
// renders the meta row with the input cell as name=value (fact 10: the
// live-test input is named `mode`).
func TestExecutionDetailShowsMetaRow(t *testing.T) {
	srv := bootedTestServer(t)

	ack := webtest.Ack(t, srv.URL, "live-test", url.Values{"mode": {"FlowDeck"}})
	id := webtest.ExecutionID(t, ack)
	body := webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)
	for _, want := range []string{
		`class="meta"`,
		"mode=FlowDeck",
		"retries: 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q: %s", want, body)
		}
	}
}

// A failed run (the failing-test run — the step always fails) has no
// failure section: the state is on the badge, the failed step is
// highlighted in the log (a row with the `state error` class), and the
// Resume button is in the actions. The first-attempt failure has no
// recovered retries — the meta shows the count only.
func TestExecutionDetailFailureNoSectionResumeInLog(t *testing.T) {
	srv := bootedTestServer(t)

	ack := webtest.Dispatch(t, srv.URL, "failing-test", nil)
	id := webtest.ExecutionID(t, ack)
	body := webtest.Detail(t, srv.URL, id, `class="badge error terminal"`)
	for _, want := range []string{
		`class="badge error terminal"`,
		"boom-step",
		`class="state error"`,
		`hx-post="/resume/` + id + `"`,
		"retries: 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q: %s", want, body)
		}
	}
	if strings.Contains(body, `class="failure"`) {
		t.Errorf("the failure section is gone — the page must not contain it: %s", body)
	}
	if strings.Contains(body, "retry 1") {
		t.Errorf("the meta shows the count only — no attempts rows: %s", body)
	}
}

// The child block under its launching step (the child-test run — the
// parent of the inner-test dispatch): the child's id as a deep link to
// /executions/{child-id}, the child's badge, and the child's step line
// (fact 10: the child's one step is named `inner`).
func TestExecutionDetailChildBlockUnderLaunchingStep(t *testing.T) {
	srv := bootedTestServer(t)

	id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "child-test", nil))
	body := webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)
	for _, want := range []string{
		`class="child-run"`,
		`class="wf-id"`,
		`class="badge success terminal"`,
		">inner<",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q: %s", want, body)
		}
	}
	if m := regexp.MustCompile(`/executions/([0-9a-f-]{36})`).FindString(body); m == "" {
		t.Errorf("page missing the child deep link: %s", body)
	}
}

// The live run shows the tail: the log's dashed tail row (the cursor ▌)
// and the path svg's dashed tail box.
func TestExecutionDetailLiveShowsTail(t *testing.T) {
	srv := bootedTestServer(t)

	id := blockRun(t, srv)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id)
	if code != http.StatusOK {
		t.Errorf("detail: %d", code)
	}
	for _, want := range []string{
		`class="tail-line"`,
		"▌",
		`class="g-node tail"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q: %s", want, body)
		}
	}
}

// An execution the core does not know is a clean 404.
func TestExecutionDetailUnknownIs404(t *testing.T) {
	srv := bootedTestServer(t)

	if code, _ := webtest.Get(t, srv.URL+"/executions/00000000-0000-0000-0000-000000000000"); code != http.StatusNotFound {
		t.Errorf("detail: got %d want %d", code, http.StatusNotFound)
	}
}

// A detail of a not-launched core is a clean 503.
func TestExecutionDetailCoreDownIs503(t *testing.T) {
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/executions/abc"); code != http.StatusServiceUnavailable {
		t.Errorf("detail: got %d want %d", code, http.StatusServiceUnavailable)
	}
}
