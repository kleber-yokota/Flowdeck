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

// The steps partial (the v6.3 split fragment) against real runs (the
// booted core, no stubs): the fragment root is the whole split (the meta
// row, the actions, the step log with the child blocks and the tail, and
// the sticky executed path); the live fragment keeps the 2s poll on the
// split root, the terminal fragment stops it.

// The fragment root is the split itself: while live it carries the 2s
// poll attributes; a terminal run has the same root without them.
func TestStepsFragmentIsTheSplit(t *testing.T) {
	srv := bootedTestServer(t)

	// Live run: the split root carries the poll attributes.
	id := blockRun(t, srv)
	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Errorf("steps: %d", code)
	}
	for _, want := range []string{
		`class="split exec-split"`,
		`hx-get="/executions/` + id + `/steps`,
		`hx-trigger="every 2s"`,
		`hx-swap="outerHTML"`,
		`hx-target="this"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment does not contain %q: %s", want, body)
		}
	}

	// Terminal run: the same root, no poll attributes.
	tid := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "live-test", nil))
	webtest.Detail(t, srv.URL, tid, `class="badge success terminal"`)
	code, body = webtest.Get(t, srv.URL+"/executions/"+tid+"/steps")
	if code != http.StatusOK {
		t.Errorf("steps: %d", code)
	}
	if !strings.Contains(body, `class="split exec-split"`) {
		t.Errorf("fragment missing the split root: %s", body)
	}
	for _, no := range []string{
		`hx-get="/executions/" + tid + "/steps"`,
		`hx-trigger="every 2s"`,
		`hx-swap="outerHTML"`,
		`hx-target="this"`,
	} {
		if strings.Contains(body, no) {
			t.Errorf("terminal fragment must not poll, but contains %q: %s", no, body)
		}
	}
}

// The fragment carries the meta row (created · elapsed · input ·
// retries: 0) and the step log: one row per completed step, each with a
// t+ start (one decimal) and the done duration; a terminal run has no
// tail row.
func TestStepsFragmentMetaAndLog(t *testing.T) {
	srv := bootedTestServer(t)

	id := webtest.ExecutionID(t, webtest.Dispatch(t, srv.URL, "linear-test", url.Values{"msg": {"FlowDeck"}}))
	webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Errorf("steps: %d", code)
	}
	for _, want := range []string{
		`class="meta"`,
		"created",
		"elapsed",
		"input: msg=FlowDeck",
		"retries: 0",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment does not contain %q: %s", want, body)
		}
	}
	for _, step := range []string{"fetch", "parse", "transform", "store"} {
		if !strings.Contains(body, ">"+step+"<") {
			t.Errorf("fragment missing the log row %q: %s", step, body)
		}
	}
	for _, re := range []*regexp.Regexp{
		// html/template escapes the t+ value's plus sign in the text node
		// (the rendered browser text stays the verbatim t+ value).
		regexp.MustCompile(`t&#43;\d+\.\d+s`),
		regexp.MustCompile(`done in \d+\.\d+s`),
	} {
		if !re.MatchString(body) {
			t.Errorf("fragment missing a %q value: %s", re, body)
		}
	}
	if strings.Contains(body, `class="tail-line"`) {
		t.Errorf("terminal fragment must not have the tail row: %s", body)
	}
}

// A live run (the block-test run): the split root carries the hx
// attributes that keep the 2s poll running.
func TestStepsPartialPollsWhenLive(t *testing.T) {
	srv := bootedTestServer(t)

	id := blockRun(t, srv)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Errorf("steps: %d", code)
	}
	for _, want := range []string{
		`class="split exec-split"`,
		`hx-get="/executions/` + id + `/steps`,
		`hx-trigger="every 2s"`,
		`hx-swap="outerHTML"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("fragment does not contain %q: %s", want, body)
		}
	}
}

// A terminal run (the live-test success run): the split root has no hx
// attributes (the poll stops) and shows the step that ran.
func TestStepsPartialTerminalStopsPolling(t *testing.T) {
	srv := bootedTestServer(t)

	id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "live-test", nil))
	webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)

	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Errorf("steps: %d", code)
	}
	if !strings.Contains(body, `class="split exec-split"`) {
		t.Errorf("fragment missing the split root: %s", body)
	}
	if !strings.Contains(body, ">live<") {
		t.Errorf("fragment must show the executed step: %s", body)
	}
	for _, no := range []string{`hx-get="/executions/" + id + "/steps"`, `hx-trigger="every 2s"`, `hx-swap="outerHTML"`, `hx-target="this"`} {
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
		t.Errorf("steps: %d", code)
	}
	fetch := strings.Index(body, ">fetch<")
	parse := strings.Index(body, ">parse<")
	transform := strings.Index(body, ">transform<")
	store := strings.Index(body, ">store<")
	if fetch < 0 || parse < 0 || transform < 0 || store < 0 {
		t.Errorf("fragment missing steps (fetch=%d parse=%d transform=%d store=%d): %s", fetch, parse, transform, store, body)
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
		t.Errorf("steps: got %d want %d", code, http.StatusNotFound)
	}
}

// The poll of a not-launched core is a clean 503 (the poll keeps waiting).
func TestStepsPartialCoreDownIs503(t *testing.T) {
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/executions/abc/steps"); code != http.StatusServiceUnavailable {
		t.Errorf("steps: got %d want %d", code, http.StatusServiceUnavailable)
	}
}
