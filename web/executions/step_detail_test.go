package executions

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/web/internal/webtest"
)

// The expand route (STEP 2): the stored output or the error of one step of
// a run, rendered as the swappable step-detail fragment.

// The completed step shows its stored output (the JSON text — the echo
// step's "fetch", quoted). html/template escapes the text node (" →
// &#34;), so the body carries the escaped form — the rendered text is the
// verbatim quoted JSON text (fact 15).
func TestStepDetailFragmentShowsOutput(t *testing.T) {
	srv := bootedTestServer(t)
	id := webtest.ExecutionID(t, webtest.Dispatch(t, srv.URL, "linear-test", url.Values{"msg": {"FlowDeck"}}))
	webtest.Detail(t, srv.URL, id, "success")
	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps/fetch")
	if code != http.StatusOK {
		t.Errorf("GET step-detail: %d, %s", code, body)
	}
	if !strings.Contains(body, `<pre class="output">&#34;fetch&#34;</pre>`) {
		t.Errorf("fragment missing the output pre: %s", body)
	}
}

// The failed step shows its error text (the fixture's pinned error).
func TestStepDetailFragmentShowsError(t *testing.T) {
	srv := bootedTestServer(t)
	id := webtest.ExecutionID(t, webtest.Dispatch(t, srv.URL, "failing-test", nil))
	webtest.Detail(t, srv.URL, id, `class="badge error terminal"`)
	code, body := webtest.Get(t, srv.URL+"/executions/"+id+"/steps/boom-step")
	if code != http.StatusOK {
		t.Errorf("GET step-detail: %d, %s", code, body)
	}
	if !strings.Contains(body, `<pre class="error">boom: step failed</pre>`) {
		t.Errorf("fragment missing the error pre: %s", body)
	}
}

// An execution the core does not know is a 404.
func TestStepDetailUnknownExecutionIs404(t *testing.T) {
	srv := bootedTestServer(t)
	code, body := webtest.Get(t, srv.URL+"/executions/00000000-0000-0000-0000-000000000000/steps/fetch")
	if code != http.StatusNotFound {
		t.Errorf("GET unknown execution: %d, %s", code, body)
	}
}

// The core not launched is a 503.
func TestStepDetailCoreDownIs503(t *testing.T) {
	srv := testServer(t, core.NewPort())
	code, body := webtest.Get(t, srv.URL+"/executions/00000000-0000-0000-0000-000000000000/steps/fetch")
	if code != http.StatusServiceUnavailable {
		t.Errorf("GET core down: %d, %s", code, body)
	}
}
