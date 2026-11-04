package executions

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/web/internal/webtest"
)

// The detail page (the execution page) renders real runs (the booted core,
// no stubs): the badge, the input values, the retries row, the executed
// path, the children list, the error and the failed step on failure, and
// the cancel / resume buttons of the live / failed / cancelled runs.

// The detail page of a live run (running — the block-test run) offers the
// Cancel button and polls (the badge and the executed path are live);
// the retries row is 0 (nothing failed yet).
func TestExecutionDetailLivePollsBadgeAndSteps(t *testing.T) {
	srv := bootedTestServer(t)

	id := blockRun(t, srv)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id)
	if code != http.StatusOK {
		t.Fatalf("detail: %d", code)
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
// executed path is static and shows the step that ran, and there is no
// retry row, no cancel and no resume.
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
		`hx-get="/executions/` + id + `/steps`,
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

// The detail page of a failed run (the failing-test run — the step always
// fails): the error badge (terminal), the error and the failed step, and
// the Resume button. The first-attempt failure has no recovered retries —
// the error badge is the last attempt, not a row.
func TestExecutionDetailFailureHasErrorAndResume(t *testing.T) {
	srv := bootedTestServer(t)

	ack := webtest.Dispatch(t, srv.URL, "failing-test", nil)
	id := webtest.ExecutionID(t, ack)
	body := webtest.Detail(t, srv.URL, id, `class="badge error terminal"`)
	for _, want := range []string{
		`class="badge error terminal"`,
		"retries: 0",
		"boom: step failed",
		`<code>boom-step</code>`,
		`hx-post="/resume/` + id + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q: %s", want, body)
		}
	}
	if strings.Contains(body, "retry 1") {
		t.Errorf("the error badge is the last attempt — no retries row: %s", body)
	}
}

// The input values render as code + the value (the struct-test run:
// Name + StartDate posted, Count defaulted to the zero value).
func TestExecutionDetailShowsInputValues(t *testing.T) {
	srv := bootedTestServer(t)

	ack := webtest.Ack(t, srv.URL, "struct-test", url.Values{
		"Name":      {"Ada"},
		"StartDate": {"2026-02-01"},
	})
	id := webtest.ExecutionID(t, ack)
	body := webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)
	for _, want := range []string{
		"<code>Name</code>: Ada",
		"<code>StartDate</code>: 2026-02-01",
		"<code>Count</code>: 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q: %s", want, body)
		}
	}
}

// The children list renders for the child-test run (the parent of the
// inner-test child dispatch); a run without children renders no list.
func TestExecutionDetailChildrenList(t *testing.T) {
	t.Run("child dispatch lists the child", func(t *testing.T) {
		srv := bootedTestServer(t)

		id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "child-test", nil))
		body := webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)
		for _, want := range []string{
			`<section class="children-section">`,
			`<a href="/executions/`,
			">inner-test</a>",
		} {
			if !strings.Contains(body, want) {
				t.Errorf("page does not contain %q: %s", want, body)
			}
		}
	})
	t.Run("no children renders no list", func(t *testing.T) {
		srv := bootedTestServer(t)

		id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "live-test", nil))
		body := webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)
		if strings.Contains(body, `children-section`) {
			t.Errorf("page must not list children: %s", body)
		}
	})
}

// An execution the core does not know is a clean 404.
func TestExecutionDetailUnknownIs404(t *testing.T) {
	srv := bootedTestServer(t)

	if code, _ := webtest.Get(t, srv.URL+"/executions/00000000-0000-0000-0000-000000000000"); code != http.StatusNotFound {
		t.Fatalf("detail: got %d want %d", code, http.StatusNotFound)
	}
}

// A detail of a not-launched core is a clean 503.
func TestExecutionDetailCoreDownIs503(t *testing.T) {
	skipBooted(t)
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/executions/abc"); code != http.StatusServiceUnavailable {
		t.Fatalf("detail: got %d want %d", code, http.StatusServiceUnavailable)
	}
}

// retriesFor: the attempts − 1 rows (the last attempt is the badge).
func TestRetriesFor(t *testing.T) {
	if got := retriesFor(1); len(got) != 0 {
		t.Errorf("retriesFor(1): got %v, want none", got)
	}
	got := retriesFor(3)
	if len(got) != 2 {
		t.Fatalf("retriesFor(3): got %d rows, want 2", len(got))
	}
	for i, want := range []int{1, 2} {
		if got[i].Number != want || got[i].Label != "failed (recovered)" || got[i].Class != "recovered" {
			t.Errorf("retriesFor(3)[%d]: got %+v", i, got[i])
		}
	}
}
