package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/core/status"
)

// GET /status is the poll half of the dispatch (phase 2): it returns the
// badge fragment for each of the four states, and only the non-terminal
// badges carry the hx attributes that keep the poll running.
func TestStatusBadgeRendersState(t *testing.T) {
	for state, want := range map[status.State]string{
		status.Queued:  `class="badge queued"`,
		status.Running: `class="badge running"`,
		status.Success: `class="badge success terminal"`,
		status.Error:   `class="badge error terminal"`,
	} {
		t.Run(string(state), func(t *testing.T) {
			d := testPort()
			d.Status = stateStatuser(state)
			srv := httptest.NewServer(New(d, okReady))
			defer srv.Close()

			code, body := get(t, srv.URL+"/status/abc")
			if code != http.StatusOK {
				t.Fatalf("status: got %d want %d", code, http.StatusOK)
			}
			if !strings.Contains(body, want) {
				t.Errorf("fragment does not contain %q: %q", want, body)
			}
			// Live badges poll and are swapped whole (outerHTML) — the old
			// badge element never wraps the new one. Terminal badges drop
			// both attributes and stop the poll.
			polling := state == status.Queued || state == status.Running
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
	for state := range map[status.State]bool{
		status.Queued:  true,
		status.Running: true,
		status.Success: true,
		status.Error:   true,
	} {
		t.Run(string(state), func(t *testing.T) {
			d := testPort()
			d.Status = stateStatuser(state)
			srv := httptest.NewServer(New(d, okReady))
			defer srv.Close()

			code, body := get(t, srv.URL+"/status/abc")
			if code != http.StatusOK {
				t.Fatalf("status: got %d want %d", code, http.StatusOK)
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

// The poll fragment is dynamic: it must never be cached by the browser.
func TestStatusBadgeNoStore(t *testing.T) {
	d := testPort()
	d.Status = stateStatuser(status.Queued)
	srv := httptest.NewServer(New(d, okReady))
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

// An ID that is not known by the core is a clean 404.
func TestStatusUnknownExecutionIs404(t *testing.T) {
	statuser := func(workflowID string) (status.State, error) {
		return "", core.ErrUnknownExecution
	}
	d := testPort()
	d.Status = statuser
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	if code, _ := get(t, srv.URL+"/status/ghost"); code != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", code, http.StatusNotFound)
	}
}

// A core that is down maps to a clean 503 (the poll keeps waiting).
func TestStatusCoreDownIs503(t *testing.T) {
	statuser := func(workflowID string) (status.State, error) {
		return "", core.ErrNotLaunched
	}
	d := testPort()
	d.Status = statuser
	srv := httptest.NewServer(New(d, okReady))
	defer srv.Close()

	if code, _ := get(t, srv.URL+"/status/abc"); code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", code, http.StatusServiceUnavailable)
	}
}
