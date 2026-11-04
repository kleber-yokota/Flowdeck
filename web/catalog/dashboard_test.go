package catalog

import (
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/web/internal/webtest"
)

// The dashboard routes against the real port (no stubs): the booted core
// (the env-driven context, the testworkflows fixtures) and, without the
// env, the real not-launched port (the 503 and the empty registry).

// The v6.3 console shell (no core state involved): the pt-BR shell, the
// live poll (5s), the wall load (once), the vendored htmx, the console line
// and the hint.
func TestRootRendersDashboardWithHTMX(t *testing.T) {
	srv := testServer(t, core.NewPort())

	status, body := webtest.Get(t, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{
		`<html lang="pt-BR">`,
		`<title>flowdeck · v6</title>`,
		`hx-get="/live"`,
		`hx-trigger="load, every 5s"`,
		`hx-get="/workflows"`,
		`<script src="/static/htmx.min.js" defer></script>`,
		`class="cmd"`,
		`class="hint"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("console does not contain %q", want)
		}
	}
}

// The live partial of the booted core: the three boxes render the real
// counts — the test creates one run per box and reads the deltas. The
// block run is enqueued before the baseline (its lifecycle — pending,
// cancelled — is part of the baseline, not a delta) and a live-test ok
// run normalizes the latest live-test before the failed-box delta (the
// delta does not depend on the previous tests' state).
func TestLivePartialBootedBoxes(t *testing.T) {
	srv := bootedTestServer(t)

	// The running box: a block-test run blocks until the cancel (enqueued
	// straight on the core — the dispatch route is the executions
	// package's, the boxes only need a run). The block run is enqueued
	// before the baseline: its own lifecycle (pending, then cancelled) is
	// part of the baseline, not a delta.
	h, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "block-test", nil)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	blockID := h.GetWorkflowID()
	// The block run occupies a worker until the cancel: the cancel is a
	// best-effort cleanup (the suite settles even when the test fails).
	t.Cleanup(func() {
		if c := boot.Ctx(); c != nil {
			_ = c.CancelWorkflow(c, blockID)
		}
	})
	// The claim takes a poll cycle: wait for the run to be running.
	deadline := time.Now().Add(30 * time.Second)
	for {
		st, err := reads.Status(blockID)
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if st == status.Running {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no running state: %s", st)
		}
		time.Sleep(200 * time.Millisecond)
	}
	// Normalize: the latest live-test run is a success, so the failed-box
	// delta below does not depend on the previous tests' state.
	hn, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "ok")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	deadline = time.Now().Add(30 * time.Second)
	for {
		st, err := reads.Status(hn.GetWorkflowID())
		if err != nil {
			t.Fatalf("status: %v", err)
		}
		if st == status.Success {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no success state: %s", st)
		}
		time.Sleep(200 * time.Millisecond)
	}
	base := liveBoxes(t, srv.URL)
	// The failed box: the latest live-test run is an error (the "err" mode).
	if _, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "err"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitLive(t, srv.URL, func(r, q, f int) bool { return r == base.running && f == base.failed+1 })
	// The boxes render: the pinned labels and the count spans.
	code, body := webtest.Get(t, srv.URL+"/live")
	if code != http.StatusOK {
		t.Fatalf("live: %d", code)
	}
	for _, want := range []string{
		`href="/runnings"`, `href="/queued"`, `href="/failed"`,
		`<span class="label">running</span>`,
		`<span class="label">queued</span>`,
		`<span class="label">cancelled · error</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("partial does not contain %q: %s", want, body)
		}
	}
	// Convergence: the cancel releases the block run (the running box loses
	// it) and the ok run puts the live-test back on success — the failed
	// box settles on the baseline plus the block-test latest-cancelled.
	if err := boot.Ctx().CancelWorkflow(boot.Ctx(), blockID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "ok"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	waitLive(t, srv.URL, func(r, q, f int) bool { return r == base.running-1 && f == base.failed+1 })
}

// The empty registry (the real not-launched port lists nothing): the empty
// message.
func TestWorkflowsPartialEmptyRegistry(t *testing.T) {
	skipBooted(t)
	srv := testServer(t, core.NewPort())

	status, body := webtest.Get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, "No workflows registered") {
		t.Errorf("empty partial does not contain the message: %q", body)
	}
}

// The live partial of a not-launched core is a clean 503 (the Fail map).
func TestLivePartialCoreDownIs503(t *testing.T) {
	skipBooted(t)
	srv := testServer(t, core.NewPort())

	if status, _ := webtest.Get(t, srv.URL+"/live"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// The wall of the booted registry: every registered workflow (no
// hardcoding), the single page "1–10 de 10", the links to the pages, and
// the base page knows nothing about specific workflows.
func TestWorkflowsPartialListsBootedRegistry(t *testing.T) {
	srv := bootedTestServer(t)

	status, body := webtest.Get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, name := range []string{"live-test", "fork-test", "linear-test", "struct-test", "noinput-test", "block-test"} {
		for _, want := range []string{
			`href="/workflows/` + name + `"`,
			`>` + name + `<`,
		} {
			if !strings.Contains(body, want) {
				t.Errorf("wall does not contain %q", want)
			}
		}
	}
	if !strings.Contains(body, "1\u201310 de 10") {
		t.Errorf("pager info: %s", body)
	}
	// Single page: no prev/next links.
	for _, no := range []string{`hx-get="/workflows?page=`} {
		if strings.Contains(body, no) {
			t.Errorf("single page with a pager link: %s", body)
		}
	}
	// The base page does not hardcode workflow names.
	_, page := webtest.Get(t, srv.URL+"/")
	for _, name := range []string{"live-test", "fork-test"} {
		if strings.Contains(page, name) {
			t.Errorf("page hardcodes %q", name)
		}
	}
}

// The fragment is dynamic: it must never be cached by the browser.
func TestWorkflowsPartialNoStore(t *testing.T) {
	srv := testServer(t, core.NewPort())

	resp, err := http.Get(srv.URL + "/workflows")
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

// The wall's page math (the pure function behind the ?page parameter):
// the page size of 20, the clamp of out-of-range pages, and the pager
// links (0 on the edge pages).
func TestWallPage(t *testing.T) {
	for _, tc := range []struct {
		total, page int
		want        [4]int // start end prev next (1-based bounds)
	}{
		{25, 1, [4]int{1, 20, 0, 2}},
		{25, 2, [4]int{21, 25, 1, 0}},
		{25, 99, [4]int{21, 25, 1, 0}},
		{25, 0, [4]int{1, 20, 0, 2}},
		{25, -3, [4]int{1, 20, 0, 2}},
		{40, 2, [4]int{21, 40, 1, 0}},
		{41, 3, [4]int{41, 41, 2, 0}},
		{10, 1, [4]int{1, 10, 0, 0}},
		{10, 5, [4]int{1, 10, 0, 0}},
		{20, 1, [4]int{1, 20, 0, 0}},
		{21, 2, [4]int{21, 21, 1, 0}},
	} {
		t.Run(fmt.Sprintf("total=%d page=%d", tc.total, tc.page), func(t *testing.T) {
			start, end, prev, next := wallPage(tc.total, fmt.Sprintf("%d", tc.page))
			got := [4]int{start + 1, end, prev, next}
			if got != tc.want {
				t.Fatalf("wallPage: got %v, want %v", got, tc.want)
			}
		})
	}
	// The invalid parameter (not a number) is page 1.
	if start, end, prev, next := wallPage(25, "abc"); [4]int{start + 1, end, prev, next} != [4]int{1, 20, 0, 2} {
		t.Fatalf("wallPage(25, abc): got %d %d %d %d, want 1 20 0 2", start, end, prev, next)
	}
}

var liveCount = regexp.MustCompile(`<span class="count">(\d+)</span>`)

type liveCounts struct{ running, queued, failed int }

// liveBoxes reads the three box counts of /live (running · queued ·
// failed).
func liveBoxes(t *testing.T, srvURL string) liveCounts {
	t.Helper()
	code, body := webtest.Get(t, srvURL+"/live")
	if code != http.StatusOK {
		t.Fatalf("live: %d, %s", code, body)
	}
	m := liveCount.FindAllStringSubmatch(body, -1)
	if len(m) != 3 {
		t.Fatalf("live: %d count spans, want 3: %s", len(m), body)
	}
	return liveCounts{mustAtoi(t, m[0][1]), mustAtoi(t, m[1][1]), mustAtoi(t, m[2][1])}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatalf("live count %q: %v", s, err)
	}
	return n
}

// waitLive polls /live until the check passes (30s deadline).
func waitLive(t *testing.T, srvURL string, ok func(r, q, f int) bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		c := liveBoxes(t, srvURL)
		if ok(c.running, c.queued, c.failed) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("live: no counts satisfying the check: got %d %d %d", c.running, c.queued, c.failed)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
