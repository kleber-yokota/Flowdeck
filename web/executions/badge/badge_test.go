package badge

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
	"flowdeck/web/internal/webtest"
)

// TestMain sets the workflows source (the testworkflows fixtures) before
// any test runs — the source graph is read at boot.
func TestMain(m *testing.M) {
	os.Exit(testcore.TestMain(m, "../../../core/testworkflows"))
}

// The badge of each state (the pure builder): the class and the label of
// the five states, and the terminal flag (the poll stops).
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
		wantID := "id-" + string(st)
		if b.WorkflowID != wantID || b.Class != want.class || b.Label != want.label || b.Terminal != want.terminal {
			t.Errorf("BadgeFor(%s): got %q %q %q %v, want %q %q %q %v",
				st, b.WorkflowID, b.Class, b.Label, b.Terminal, wantID, want.class, want.label, want.terminal)
		}
	}
}

// Row builds the runline row of a listed run: the badge of its state,
// the elapsed, and the row actions — cancelable on the live states only,
// resumeable on the error state only.
func TestRowOfEachState(t *testing.T) {
	created := time.Now().Add(-time.Minute)
	for st, want := range map[status.State]struct {
		cancelable, resumeable bool
	}{
		status.Queued:    {true, false},
		status.Running:   {true, false},
		status.Success:   {false, false},
		status.Error:     {false, true},
		status.Cancelled: {false, false},
	} {
		r := Row(reads.Execution{ID: "id-" + string(st), State: st, CreatedAt: created})
		if r.Cancelable != want.cancelable || r.Resumeable != want.resumeable {
			t.Errorf("Row(%s): Cancelable=%v Resumeable=%v, want %v %v",
				st, r.Cancelable, r.Resumeable, want.cancelable, want.resumeable)
		}
		if r.WorkflowID != "id-"+string(st) || r.Class != string(st) || r.Terminal != st.Terminal() {
			t.Errorf("Row(%s) badge: got %q %q %v", st, r.WorkflowID, r.Class, r.Terminal)
		}
	}
}

// ElapsedLabel renders the elapsed time: under a minute in seconds, from
// a minute up in minutes and zero-padded seconds (the boundary is the
// minute itself — 60s is already the minute form).
func TestElapsedLabel(t *testing.T) {
	for secs, want := range map[int64]string{
		0:    "0s",
		42:   "42s",
		59:   "59s",
		60:   "1m 00s",
		61:   "1m 01s",
		125:  "2m 05s",
		3661: "61m 01s",
	} {
		if got := ElapsedLabel(time.Duration(secs) * time.Second); got != want {
			t.Errorf("ElapsedLabel(%ds): got %q want %q", secs, got, want)
		}
	}
}

// The poll fragment against the booted core: a real run of each state,
// the one badge element the page swaps in — the live badges carry the
// hx attributes that keep the 2s poll running, the terminal badges are
// static and stop it.
func TestHandleBadgeOfABootedCore(t *testing.T) {
	testcore.BootTest(t, "../../../core/testworkflows")
	p := core.NewPort()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status/{id}", func(w http.ResponseWriter, r *http.Request) {
		HandleBadge(w, r, p)
	})
	srv := webtest.TestServer(t, mux)

	// The deterministic fixtures: the frozen queue (the executor never
	// claims the run — the queued state) and the live-test run (success).
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue frozen: %v", err)
	}
	code, frag := webtest.Get(t, srv.URL+"/status/"+h.GetWorkflowID())
	if code != http.StatusOK || !strings.Contains(frag, `class="badge queued"`) {
		t.Fatalf("queued badge: %d, %s", code, frag)
	}
	if !strings.Contains(frag, `hx-trigger="every 2s"`) || !strings.Contains(frag, `hx-swap="outerHTML"`) {
		t.Fatalf("queued badge without the poll attributes: %s", frag)
	}

	ex, err := p.Dispatch.Enqueue("live-test", nil)
	if err != nil {
		t.Fatalf("Dispatch: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll: %v", err)
	}
	code, frag = webtest.Get(t, srv.URL+"/status/"+ex.ID)
	if code != http.StatusOK || !strings.Contains(frag, `class="badge success terminal"`) {
		t.Fatalf("success badge: %d, %s", code, frag)
	}
	if strings.Contains(frag, "hx-trigger") || strings.Contains(frag, "hx-swap") {
		t.Fatalf("terminal badge with the poll attributes: %s", frag)
	}
}

// An ID the core does not know is a clean 404.
func TestHandleBadgeOfAnUnknownExecutionIs404(t *testing.T) {
	testcore.BootTest(t, "../../../core/testworkflows")
	p := core.NewPort()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /status/{id}", func(w http.ResponseWriter, r *http.Request) {
		HandleBadge(w, r, p)
	})
	srv := webtest.TestServer(t, mux)

	if code, _ := webtest.Get(t, srv.URL+"/status/00000000-0000-0000-0000-000000000000"); code != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", code, http.StatusNotFound)
	}
}
