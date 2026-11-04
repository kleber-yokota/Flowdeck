package executions

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/web/internal/webtest"
)

// The dispatch (the POST half) against the booted core: the ACK row with
// the badge (that starts the poll), the first execution date and the link
// to the detail page — the run is a real run of the deterministic
// fixtures.

// The dispatch ACK: badge queued + date + detail link — no form in the
// ACK (the form lives on the workflow page).
func TestDispatchACKShowsBadgeAndLink(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := webtest.PostForm(t, srv.URL+"/run/linear-test", url.Values{"msg": {"ping"}})
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	body := webtest.ReadAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch: %d, %s", resp.StatusCode, body)
	}
	id := webtest.ExecutionID(t, body)
	for _, want := range []string{
		`class="badge queued"`,
		`class="exec-date">`,
		`href="/executions/` + id + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("ACK does not contain %q: %s", want, body)
		}
	}
	if strings.Contains(body, `<form`) {
		t.Errorf("ACK contains a form: %s", body)
	}
}

// A dispatch of a workflow that is not registered is a clean 404 (the
// registry is the source — no hardcoding).
func TestDispatchUnknownWorkflowIs404(t *testing.T) {
	srv := testServer(t, core.NewPort())

	resp, err := webtest.Post(t, srv.URL+"/run/ghost")
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("dispatch: got %d want %d", resp.StatusCode, http.StatusNotFound)
	}
}

// The dispatch ACK is dynamic: it must never be cached by the browser.
func TestDispatchNoStore(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := webtest.PostForm(t, srv.URL+"/run/noinput-test", nil)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

// The dispatch of a not-launched core is a clean 503.
func TestDispatchCoreDownIs503(t *testing.T) {
	skipBooted(t)
	srv := testServer(t, core.NewPort())

	resp, err := webtest.Post(t, srv.URL+"/run/noinput-test")
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("dispatch: got %d want %d", resp.StatusCode, http.StatusServiceUnavailable)
	}
}
