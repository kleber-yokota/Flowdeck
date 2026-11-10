package catalog

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/web/executions/badge"
	"flowdeck/web/internal/webtest"
)

// The runnings routes against the real port (no stubs): the booted core
// (the env-driven context, the testworkflows fixtures) and, without the
// env, the real not-launched port (the 503 path). The shell has no data
// (both modes); the partial carries the running runs (booted).

// The runnings shell (no port call — the list is the partial's poll): the
// pt-BR shell, the console line, the 5s poll on #running-list, and the
// back link.
func TestRunningPageRendersShell(t *testing.T) {
	srv := testServer(t, core.NewPort())

	code, body := webtest.Get(t, srv.URL+"/runnings")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	for _, want := range []string{
		`<html lang="pt-BR">`,
		`<title>flowdeck · v6 · running</title>`,
		`<link rel="stylesheet" href="/static/style.css">`,
		`<script src="/static/htmx.min.js" defer></script>`,
		`class="frame"`,
		`id="running-list"`,
		`hx-get="/runnings/list"`,
		`hx-trigger="load, every 5s"`,
		`workflow · id · started · status · elapsed`,
		`atualização a cada 5s · cada linha → a página da execução`,
		`href="/"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shell does not contain %q", want)
		}
	}
}

var (
	tsRe      = regexp.MustCompile(`class="ts">[0-9]{2}:[0-9]{2}<`)
	elapsedRe = regexp.MustCompile(`class="elapsed">(\d+s|\d+m \d\d?s)<`)
)

// The runnings partial of the booted core: one row per running run (no
// dedup by workflow name — two live-test block fires are two rows · F3 ·
// F8), the pinned row markup, and the settle (the suite contract · F2).
func TestRunningListRows(t *testing.T) {
	srv := bootedTestServer(t)

	// The two block fires: each run blocks until the cancel (F8). The
	// cancels are best-effort cleanups (the suite settles even when the
	// test fails).
	ids := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		h, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "block")
		if err != nil {
			t.Errorf("enqueue: %v", err)
			return
		}
		id := h.GetWorkflowID()
		ids = append(ids, id)
		t.Cleanup(func() {
			if c := boot.Ctx(); c != nil {
				_ = c.CancelWorkflow(c, id)
			}
		})
	}
	// The claim takes a poll cycle: wait for both runs to be running.
	for _, id := range ids {
		deadline := time.Now().Add(30 * time.Second)
		for {
			st, err := reads.Status(id)
			if err != nil {
				t.Errorf("status: %v", err)
				return
			}
			if st == status.Running {
				break
			}
			if time.Now().After(deadline) {
				t.Errorf("no running state: %s", st)
				return
			}
			time.Sleep(200 * time.Millisecond)
		}
	}

	code, body := webtest.Get(t, srv.URL+"/runnings/list")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	if got := strings.Count(body, `class="exec-row"`); got != 2 {
		t.Errorf("rows: got %d, want 2: %s", got, body)
	}
	if got := strings.Count(body, `class="wf-name">live-test<`); got != 2 {
		t.Errorf("names: got %d, want 2: %s", got, body)
	}
	if got := strings.Count(body, `class="status running">running<`); got != 2 {
		t.Errorf("statuses: got %d, want 2: %s", got, body)
	}
	for _, id := range ids {
		if !strings.Contains(body, `href="/executions/`+id+`"`) {
			t.Errorf("row for %s missing: %s", id, body)
		}
	}
	if got := len(tsRe.FindAllString(body, -1)); got != 2 {
		t.Errorf("started: got %d, want 2: %s", got, body)
	}
	if got := len(elapsedRe.FindAllString(body, -1)); got != 2 {
		t.Errorf("elapsed: got %d, want 2: %s", got, body)
	}

	// The settle (the suite contract · F2): the two cancels release the
	// block runs (the cancel is the release · F8), and one ok fire leaves
	// the latest live-test run in SUCCESS.
	for _, id := range ids {
		if err := boot.Ctx().CancelWorkflow(boot.Ctx(), id); err != nil {
			t.Errorf("cancel: %v", err)
			return
		}
	}
	ho, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "ok")
	if err != nil {
		t.Errorf("enqueue: %v", err)
		return
	}
	deadline := time.Now().Add(30 * time.Second)
	for {
		st, err := reads.Status(ho.GetWorkflowID())
		if err != nil {
			t.Errorf("status: %v", err)
			return
		}
		if st == status.Success {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("no success state: %s", st)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
	_, body = webtest.Get(t, srv.URL+"/runnings/list")
	if !strings.Contains(body, "no runs") {
		t.Errorf("settled partial does not contain %q: %s", "no runs", body)
	}
}

// The empty state of the booted partial: the suite is quiet at the
// boundary (F2) — poll until the list settles on "no runs".
func TestRunningListEmptyState(t *testing.T) {
	srv := bootedTestServer(t)

	deadline := time.Now().Add(30 * time.Second)
	for {
		code, body := webtest.Get(t, srv.URL+"/runnings/list")
		if code != http.StatusOK {
			t.Errorf("status: got %d want %d", code, http.StatusOK)
			return
		}
		if strings.Contains(body, "no runs") {
			if strings.Contains(body, `class="exec-row"`) {
				t.Errorf("empty state with a row: %s", body)
				return
			}
			return
		}
		if time.Now().After(deadline) {
			t.Errorf("no empty state: %s", body)
			return
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// The runnings partial of a not-launched core is a clean 503 (the Fail
// map: boot.ErrNotLaunched → "core not ready").
func TestRunningListNotLaunchedIs503(t *testing.T) {
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/runnings/list"); code != http.StatusServiceUnavailable {
		t.Errorf("status: got %d want %d", code, http.StatusServiceUnavailable)
	}
}

// The partial is dynamic: it must never be cached by the browser (the 5s
// poll).
func TestRunningListNoStore(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := http.Get(srv.URL + "/runnings/list")
	if err != nil {
		t.Errorf("GET: %v", err)
		return
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Errorf("read body: %v", err)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

// The row mapping (the pure function behind the partial): the started
// time (15:04) and the elapsed from the RFC3339 CreatedAt (F4); a
// malformed CreatedAt is an error (not a guess).
func TestRunningRows(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 4, 39, 0, time.UTC)
	got, err := runningRows([]reads.LiveRun{{Name: "live-test", ID: "wf_1", State: "running", CreatedAt: "2026-11-04T12:04:05Z"}}, now)
	if err != nil {
		t.Errorf("runningRows: %v", err)
		return
	}
	want := runningRow{Name: "live-test", ID: "wf_1", Started: "12:04", Elapsed: "34s"}
	if len(got) != 1 || got[0] != want {
		t.Errorf("rows: got %v, want %v", got, want)
	}
	// The malformed CreatedAt is an error (not a guess).
	if _, err := runningRows([]reads.LiveRun{{Name: "live-test", ID: "wf_2", State: "running", CreatedAt: "not-a-time"}}, now); err == nil {
		t.Errorf("malformed created_at: no error")
	}
	// The elapsed format: <60s "42s" · ≥60s "%dm %02ds" (the helper now
	// lives in the executions package — the catalog's call site).
	for _, tc := range []struct {
		d    time.Duration
		want string
	}{
		{0, "0s"},
		{42 * time.Second, "42s"},
		{59 * time.Second, "59s"},
		{time.Minute, "1m 00s"},
		{72 * time.Second, "1m 12s"},
		{122 * time.Second, "2m 02s"},
	} {
		if got := badge.ElapsedLabel(tc.d); got != tc.want {
			t.Errorf("ElapsedLabel(%s): got %q want %q", tc.d, got, tc.want)
		}
	}
}
