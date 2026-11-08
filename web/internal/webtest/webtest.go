// Package webtest is the shared HTTP helpers of the web handler tests:
// the web is only tested against the booted core (the env-driven context
// and the testworkflows fixtures) or the real not-launched port (the 503
// and empty-state paths) — there is no fake Port.
package webtest

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// TestServer serves the handler over HTTP for the duration of the test:
// the close is the first cleanup registered on t — it runs last (LIFO), so
// the handler is alive until every cleanup registered later has run.
func TestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close) // registered first on t: runs last (LIFO)
	return srv
}

// Get issues a GET and returns the status code and the body as a string.
func Get(t *testing.T, url string) (int, string) {
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

// Post issues a POST and returns the open response (the caller reads and
// closes it).
func Post(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	return http.Post(url, "application/octet-stream", nil)
}

// PostForm posts a urlencoded form and returns the open response (the
// caller closes it).
func PostForm(t *testing.T, url string, form url.Values) (*http.Response, error) {
	t.Helper()
	return http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
}

// ReadAll reads the whole response body.
func ReadAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// Dispatch posts a dispatch (form nil for the no-input dispatch) and
// returns the ACK row fragment (the 200 response).
func Dispatch(t *testing.T, srvURL, name string, form url.Values) string {
	t.Helper()
	resp, err := PostForm(t, srvURL+"/run/"+name, form)
	if err != nil {
		t.Fatalf("POST /run/%s: %v", name, err)
	}
	body := ReadAll(t, resp)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch %s: %d, %s", name, resp.StatusCode, body)
	}
	return body
}

// Ack is the dispatch of a run that stays live (the deterministic
// fixtures): the ACK row is the queued badge row with the new workflow ID.
func Ack(t *testing.T, srvURL, name string, form url.Values) string {
	t.Helper()
	body := Dispatch(t, srvURL, name, form)
	if !strings.Contains(body, `class="badge queued"`) {
		t.Fatalf("dispatch %s: no queued badge: %s", name, body)
	}
	return body
}

// Detail polls GET /executions/{id} until the page contains frag (30s
// deadline) and returns the final page body.
func Detail(t *testing.T, srvURL, id, frag string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	var code int
	var body string
	for {
		code, body = Get(t, srvURL+"/executions/"+id)
		if code == http.StatusOK && strings.Contains(body, frag) {
			return body
		}
		if time.Now().After(deadline) {
			t.Fatalf("detail %s: no %q: %d, %s", id, frag, code, body)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// ExecutionID extracts the workflow ID from an execution row (the dispatch
// ACK).
func ExecutionID(t *testing.T, row string) string {
	t.Helper()
	const prefix = `href="/executions/`
	idx := strings.Index(row, prefix)
	if idx < 0 {
		t.Fatalf("row without an execution link: %q", row)
	}
	start := idx + len(prefix)
	end := start + strings.Index(row[start:], `"`)
	return row[start:end]
}

// ContainsAll reports whether s contains every frag.
func ContainsAll(s string, frags ...string) bool {
	for _, frag := range frags {
		if !strings.Contains(s, frag) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether s contains at least one frag.
func ContainsAny(s string, frags ...string) bool {
	for _, frag := range frags {
		if strings.Contains(s, frag) {
			return true
		}
	}
	return false
}
