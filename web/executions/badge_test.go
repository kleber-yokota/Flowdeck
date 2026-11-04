package executions

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/status"
	"flowdeck/web/internal/webtest"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// The status badge (the poll half of the dispatch) against the booted
// core: each state is a real run (the deterministic fixtures) and the
// fragment is the one the page swaps in. Only the non-terminal badges
// carry the hx attributes that keep the 2s poll running.

// The badge of each state, from a real run of that state:
//   - queued: the frozen queue (worker concurrency 0 — the executor never
//     claims the run: the deterministic queued state)
//   - running: the block-test run (the deterministic Running state)
//   - success: the live-test run (the default mode)
//   - error: the live-test "err" run
//   - cancelled: the block-test run cancelled by this test
func TestStatusBadgeRendersState(t *testing.T) {
	for state, want := range map[status.State]string{
		status.Queued:    `class="badge queued"`,
		status.Running:   `class="badge running"`,
		status.Success:   `class="badge success terminal"`,
		status.Error:     `class="badge error terminal"`,
		status.Cancelled: `class="badge cancelled terminal"`,
	} {
		t.Run(string(state), func(t *testing.T) {
			srv := bootedTestServer(t)

			id := runInState(t, srv, state)
			_, frag := webtest.Get(t, srv.URL+"/status/"+id)
			if !strings.Contains(frag, want) {
				t.Fatalf("fragment does not contain %q: %q", want, frag)
			}
			// Live badges poll and are swapped whole (outerHTML) — the old
			// badge element never wraps the new one. Terminal badges drop
			// both attributes and stop the poll.
			polling := state == status.Queued || state == status.Running
			for attr, wantAttr := range map[string]bool{
				`hx-trigger="every 2s"`: polling,
				`hx-swap="outerHTML"`:   polling,
			} {
				if has := strings.Contains(frag, attr); has != wantAttr {
					t.Errorf("%s present: got %v want %v: %q", attr, has, wantAttr, frag)
				}
			}
			// The fragment must be exactly one complete badge element, not
			// a bare text fragment: htmx swaps it in with outerHTML, which
			// replaces the old badge element entirely. If the fragment were
			// not a single root element, the new badge would be nested
			// inside the old one and the old state's outline would stay
			// visible — the "silhouette" bug.
			if !strings.HasPrefix(frag, `<span class="badge`) || !strings.HasSuffix(frag, `</span>`) {
				t.Errorf("fragment is not a single span root: %q", frag)
			}
		})
	}
}

// runInState creates a real run in the given state (the deterministic
// fixtures) and settles it via t.Cleanup (the live runs are cancelled,
// the error run is followed by an ok dispatch) — the suite converges
// regardless of test order.
func runInState(t *testing.T, srv *httptest.Server, state status.State) string {
	t.Helper()
	switch state {
	case status.Queued:
		// The frozen queue (worker concurrency 0): the executor never
		// claims the run — it stays ENQUEUED.
		if _, err := dbos.RegisterQueue(boot.Ctx(), "frozen", dbos.WithWorkerConcurrency(0)); err != nil {
			t.Fatalf("frozen queue: %v", err)
		}
		h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "noinput-test", nil)
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		id := h.GetWorkflowID()
		t.Cleanup(func() { cancelRun(t, srv, id) })
		return id
	case status.Running:
		id := blockRun(t, srv)
		// The claim takes a poll cycle: wait for the run to be running
		// (block-test stays running until cancelled — deterministic).
		pollFragment(t, srv, id, `class="badge running"`)
		return id
	case status.Success:
		ack := webtest.Ack(t, srv.URL, "live-test", nil)
		id := webtest.ExecutionID(t, ack)
		webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)
		return id
	case status.Error:
		ack := webtest.Ack(t, srv.URL, "live-test", url.Values{"mode": {"err"}})
		id := webtest.ExecutionID(t, ack)
		webtest.Detail(t, srv.URL, id, `class="badge error terminal"`)
		// Convergence: the live-test back on success (the failed box reads
		// the latest run per workflow).
		t.Cleanup(func() { webtest.Ack(t, srv.URL, "live-test", nil) })
		return id
	case status.Cancelled:
		id := blockRun(t, srv)
		cancelRun(t, srv, id)
		webtest.Detail(t, srv.URL, id, `class="badge cancelled terminal"`)
		return id
	default:
		t.Fatalf("state %s is not a fixture state", state)
		return ""
	}
}

// The badge of a state (the pure builder): the class and the label of the
// five states, and the terminal flag (the poll stops).
func TestBadgeForStates(t *testing.T) {
	for st, want := range map[status.State]struct {
		class, label string
		terminal     bool
	}{
		status.Queued:    {"queued", "Queued", false},
		status.Running:   {"running", "Running", false},
		status.Success:   {"success", "Success", true},
		status.Error:     {"error", "Error", true},
		status.Cancelled: {"cancelled", "Cancelled", true},
	} {
		b := BadgeFor("id-"+string(st), st)
		if b.Class != want.class || b.Label != want.label || b.Terminal != want.terminal {
			t.Errorf("BadgeFor(%s): got %q %q %v, want %q %q %v", st, b.Class, b.Label, b.Terminal, want.class, want.label, want.terminal)
		}
	}
}

// cancelRun cancels a real run (the ACK is the cancelled badge) and
// settles the response.
func cancelRun(t *testing.T, srv *httptest.Server, id string) {
	t.Helper()
	resp, err := webtest.Post(t, srv.URL+"/cancel/"+id)
	if err != nil {
		t.Fatalf("cancel %s: %v", id, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("cancel %s: %d", id, resp.StatusCode)
	}
}

// pollFragment polls /status/{id} until the fragment contains want (30s
// deadline).
func pollFragment(t *testing.T, srv *httptest.Server, id, want string) string {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	_, frag := webtest.Get(t, srv.URL+"/status/"+id)
	for !strings.Contains(frag, want) {
		if time.Now().After(deadline) {
			t.Fatalf("status %s: no %q: %s", id, want, frag)
		}
		time.Sleep(200 * time.Millisecond)
		_, frag = webtest.Get(t, srv.URL+"/status/"+id)
	}
	return frag
}

// The poll fragment is dynamic: it must never be cached by the browser.
func TestStatusBadgeNoStore(t *testing.T) {
	srv := bootedTestServer(t)

	id := webtest.ExecutionID(t, webtest.Ack(t, srv.URL, "live-test", nil))
	webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)

	resp, err := http.Get(srv.URL + "/status/" + id)
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
	srv := bootedTestServer(t)

	if code, _ := webtest.Get(t, srv.URL+"/status/00000000-0000-0000-0000-000000000000"); code != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", code, http.StatusNotFound)
	}
}

// The poll of a not-launched core is a clean 503 (the poll keeps waiting).
func TestStatusCoreDownIs503(t *testing.T) {
	skipBooted(t)
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/status/abc"); code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", code, http.StatusServiceUnavailable)
	}
}
