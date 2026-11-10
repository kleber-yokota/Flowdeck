package catalog

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/board"
	"flowdeck/core/boot"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
	"flowdeck/web/internal/webtest"
)

// The queued routes against the real port (no stubs): the booted core
// (the per-test boot · the testworkflows fixtures) and, without the boot,
// the real not-launched port (the 503 path). The shell has no data (both
// modes); the partial carries the queued runs (booted).

// The queued shell (no port call — the list is the partial's poll): the
// pt-BR shell, the console line, the 5s poll on #queued-list, and the
// back link.
func TestQueuedPageRendersShell(t *testing.T) {
	srv := testServer(t, core.NewPort())

	code, body := webtest.Get(t, srv.URL+"/queued")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	for _, want := range []string{
		`<html lang="pt-BR">`,
		`<title>flowdeck · v6 · queued</title>`,
		`<link rel="stylesheet" href="/static/style.css">`,
		`<script src="/static/htmx.min.js" defer></script>`,
		`class="frame"`,
		`id="queued-list"`,
		`hx-get="/queued/list"`,
		`hx-trigger="load, every 5s"`,
		`workflow · id · queued since · status · wait`,
		`atualização a cada 5s · cada linha → a página da execução`,
		`href="/"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shell does not contain %q", want)
		}
	}
}

// The queued partial of the booted core: one row per queued run (no dedup
// by workflow name — two live-test ok fires on live-frozen are two rows ·
// F2 · R7 · R8), the pinned row markup, and the settle (the suite
// contract · F3).
func TestQueuedListRows(t *testing.T) {
	srv := bootedTestServer(t)

	// The two ok fires on the frozen queue: the executor never claims
	// them (the status is queued deterministically · F2 · R8). The
	// cancels are best-effort cleanups (the suite settles even when the
	// test fails).
	ids := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		h, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}
		id := h.GetWorkflowID()
		ids = append(ids, id)
		t.Cleanup(func() {
			if c := boot.Ctx(); c != nil {
				_ = c.CancelWorkflow(c, id)
			}
		})
	}
	for _, id := range ids {
		if _, err := testcore.PollState(t, id, status.Queued); err != nil {
			t.Fatalf("poll to queued: %v", err)
		}
	}

	code, body := webtest.Get(t, srv.URL+"/queued/list")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	if got := strings.Count(body, `class="exec-row"`); got != 2 {
		t.Errorf("rows: got %d, want 2: %s", got, body)
	}
	if got := strings.Count(body, `class="wf-name">live-test<`); got != 2 {
		t.Errorf("names: got %d, want 2: %s", got, body)
	}
	if got := strings.Count(body, `class="status queued">queued<`); got != 2 {
		t.Errorf("statuses: got %d, want 2: %s", got, body)
	}
	for _, id := range ids {
		if !strings.Contains(body, `href="/executions/`+id+`"`) {
			t.Errorf("row for %s missing: %s", id, body)
		}
	}
	if got := len(tsRe.FindAllString(body, -1)); got != 2 {
		t.Errorf("queued since: got %d, want 2: %s", got, body)
	}
	if got := len(elapsedRe.FindAllString(body, -1)); got != 2 {
		t.Errorf("wait: got %d, want 2: %s", got, body)
	}

	// The settle (the suite contract · F3): the two cancels + one ok fire
	// on the main queue leaves the latest live-test run in SUCCESS, and
	// the partial settles on the empty state.
	for _, id := range ids {
		if err := boot.Ctx().CancelWorkflow(boot.Ctx(), id); err != nil {
			t.Fatalf("cancel: %v", err)
		}
	}
	ho, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "ok")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ho.GetWorkflowID(), status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	_, body = webtest.Get(t, srv.URL+"/queued/list")
	if !strings.Contains(body, "no runs") {
		t.Errorf("settled partial does not contain %q: %s", "no runs", body)
	}
}

// The empty state of the booted partial: the suite is quiet at the
// boundary (F3) — poll until the list settles on "no runs".
func TestQueuedListEmptyState(t *testing.T) {
	srv := bootedTestServer(t)

	deadline := time.Now().Add(30 * time.Second)
	for {
		code, body := webtest.Get(t, srv.URL+"/queued/list")
		if code != http.StatusOK {
			t.Fatalf("status: got %d want %d", code, http.StatusOK)
		}
		if strings.Contains(body, "no runs") {
			if strings.Contains(body, `class="exec-row"`) {
				t.Fatalf("empty state with a row: %s", body)
			}
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("no empty state: %s", body)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// The queued partial of a not-launched core is a clean 503 (the Fail map:
// boot.ErrNotLaunched → "core not ready").
func TestQueuedListNotLaunchedIs503(t *testing.T) {
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/queued/list"); code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", code, http.StatusServiceUnavailable)
	}
}

// The partial is dynamic: it must never be cached by the browser (the 5s
// poll).
func TestQueuedListNoStore(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := http.Get(srv.URL + "/queued/list")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

// The row mapping (the pure function behind the partial): the queued
// since time (15:04) and the wait from the RFC3339 CreatedAt (F4); a
// malformed CreatedAt is an error (not a guess).
func TestQueuedRows(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 4, 39, 0, time.UTC)
	got, err := queuedRows([]board.LiveRun{{Name: "live-test", ID: "wf_1", State: "queued", CreatedAt: "2026-11-04T12:04:05Z"}}, now)
	if err != nil {
		t.Fatalf("queuedRows: %v", err)
	}
	want := queuedRow{Name: "live-test", ID: "wf_1", Started: "12:04", Elapsed: "34s"}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("rows: got %v, want %v", got, want)
	}
	// The malformed CreatedAt is an error (not a guess).
	if _, err := queuedRows([]board.LiveRun{{Name: "live-test", ID: "wf_2", State: "queued", CreatedAt: "not-a-time"}}, now); err == nil {
		t.Fatal("malformed created_at: no error")
	}
}
