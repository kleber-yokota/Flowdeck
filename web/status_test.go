package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flowdeck/core"
)

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
