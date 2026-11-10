package catalog

import (
	"io"
	"net/http"
	"regexp"
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

// The failed routes against the real port (no stubs): the booted core
// (the per-test boot · the testworkflows fixtures) and, without the boot,
// the real not-launched port (the 503 path). The shell has no data (both
// modes); the partial carries the failed runs (booted). Every fired run is
// driven to a terminal state by its owning test (the error · the cancel ·
// the ok re-fire) — the fresh per-test sqlite file (the per-test boot)
// isolates the terminal failing-test ERROR row the booted tests contribute.

// the failed ts column (the "02/01 15:04" format of the partial).
var failedTSRe = regexp.MustCompile(`\d{2}/\d{2} \d{2}:\d{2}`)

// The failed shell (no port call — the list is the partial's poll): the
// pt-BR shell, the console line, the 5s poll on #failed-list, and the back
// link.
func TestFailedPageRendersShell(t *testing.T) {
	srv := testServer(t, core.NewPort())

	code, body := webtest.Get(t, srv.URL+"/failed")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	for _, want := range []string{
		`<html lang="pt-BR">`,
		`<title>flowdeck · v6 · failed</title>`,
		`<link rel="stylesheet" href="/static/style.css">`,
		`<script src="/static/htmx.min.js" defer></script>`,
		`class="frame"`,
		`id="failed-list"`,
		`hx-get="/failed/list"`,
		`hx-trigger="load, every 5s"`,
		`workflow · id · ts · status · message`,
		`atualização a cada 5s · cada linha → a página da execução (o step com o erro fica destacado)`,
		`href="/"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("shell does not contain %q", want)
		}
	}
}

// The failed partial of the booted core: one row per failed workflow (the
// latest run error ∪ cancelled) — the error row (the batched message) and
// the cancelled row (the empty message · no trace), and the settle (the
// latest success removes the workflow's row · the failing-test row remains
// — the terminal error is the test's own contribution · the fresh
// per-test file isolates it).
func TestFailedListRows(t *testing.T) {
	srv := bootedTestServer(t)

	// The error row: the failing-test fire (the step always fails — the
	// badge settles on the terminal error).
	h, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "failing-test", nil)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	id := h.GetWorkflowID()
	if _, err := testcore.PollState(t, id, status.Error); err != nil {
		t.Fatalf("poll to error: %v", err)
	}

	code, body := webtest.Get(t, srv.URL+"/failed/list")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	for _, want := range []string{
		`class="wf-name">failing-test<`,
		`class="status error">error<`,
		`boom: step failed`,
		`href="/executions/` + id + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("partial does not contain %q: %s", want, body)
		}
	}
	if got := len(failedTSRe.FindAllString(body, -1)); got != 1 {
		t.Errorf("ts: got %d, want 1: %s", got, body)
	}

	// The cancelled row: the live-test "ok" fire on live-frozen (the frozen
	// queue — the executor never claims it) is cancelled in place.
	ch, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	ca := ch.GetWorkflowID()
	if _, err := testcore.PollState(t, ca, status.Queued); err != nil {
		t.Fatalf("poll to queued: %v", err)
	}
	if err := boot.Ctx().CancelWorkflow(boot.Ctx(), ca); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := testcore.PollState(t, ca, status.Cancelled); err != nil {
		t.Fatalf("poll to cancelled: %v", err)
	}
	code, body = webtest.Get(t, srv.URL+"/failed/list")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	for _, want := range []string{
		`class="status cancelled">cancelled<`,
		`class="message"></span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("partial does not contain %q: %s", want, body)
		}
	}

	// The settle: the live-test "ok" fire on the main queue leaves the
	// latest live-test run in SUCCESS — the live-test row is gone.
	ho, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "ok")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ho.GetWorkflowID(), status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	code, body = webtest.Get(t, srv.URL+"/failed/list")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	if !strings.Contains(body, `class="wf-name">failing-test<`) {
		t.Errorf("the failing-test row is gone: %s", body)
	}
	if strings.Contains(body, `class="wf-name">live-test<`) {
		t.Errorf("the live-test row remains after the latest success: %s", body)
	}
}

// The empty state of the booted partial: the fresh per-test file is quiet
// at the boundary — poll until the list settles on "no failed runs".
func TestFailedListEmptyState(t *testing.T) {
	srv := bootedTestServer(t)

	deadline := time.Now().Add(30 * time.Second)
	for {
		code, body := webtest.Get(t, srv.URL+"/failed/list")
		if code != http.StatusOK {
			t.Fatalf("status: got %d want %d", code, http.StatusOK)
		}
		if strings.Contains(body, "no failed runs") {
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

// The failed partial of a not-launched core is a clean 503 (the Fail map:
// boot.ErrNotLaunched → "core not ready").
func TestFailedListNotLaunchedIs503(t *testing.T) {
	srv := testServer(t, core.NewPort())

	if code, _ := webtest.Get(t, srv.URL+"/failed/list"); code != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", code, http.StatusServiceUnavailable)
	}
}

// The partial is dynamic: it must never be cached by the browser (the 5s
// poll).
func TestFailedListNoStore(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := http.Get(srv.URL + "/failed/list")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

// The row mapping (the pure function behind the partial): the ts
// ("02/01 15:04") from the RFC3339 CreatedAt, the status (the row's state)
// and the message copied from the row; a malformed CreatedAt is an error
// (not a guess).
func TestFailedRows(t *testing.T) {
	now := time.Date(2026, 11, 4, 12, 4, 39, 0, time.UTC)
	got, err := failedRows([]board.FailedRun{{
		Name: "failing-test", ID: "wf_1", State: "error",
		CreatedAt: "2026-11-04T12:04:05Z", Message: "boom: step failed",
	}}, now)
	if err != nil {
		t.Fatalf("failedRows: %v", err)
	}
	want := failedRow{Name: "failing-test", ID: "wf_1", TS: "04/11 12:04", Status: "error", Message: "boom: step failed"}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("rows: got %v, want %v", got, want)
	}
	// The malformed CreatedAt is an error (not a guess).
	if _, err := failedRows([]board.FailedRun{{
		Name: "failing-test", ID: "wf_2", State: "cancelled",
		CreatedAt: "not-a-time", Message: "",
	}}, now); err == nil {
		t.Fatal("malformed created_at: no error")
	}
}

// The pager clamp (the wallPage reuse): three failed rows and ?page=99 is
// the last page (page 1 — three rows fit the 20 limit): the info "1–3 de 3"
// and no prev/next links (the edge page).
func TestFailedListPagerClamp(t *testing.T) {
	srv := bootedTestServer(t)

	// The three failed rows: the failing-test error · the cancelled
	// live-test · the cancelled block-test.
	er, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "failing-test", nil)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, er.GetWorkflowID(), status.Error); err != nil {
		t.Fatalf("poll to error: %v", err)
	}

	ch, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	ca := ch.GetWorkflowID()
	if _, err := testcore.PollState(t, ca, status.Queued); err != nil {
		t.Fatalf("poll to queued: %v", err)
	}
	if err := boot.Ctx().CancelWorkflow(boot.Ctx(), ca); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := testcore.PollState(t, ca, status.Cancelled); err != nil {
		t.Fatalf("poll to cancelled: %v", err)
	}

	bh, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "block-test", nil)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	blockID := bh.GetWorkflowID()
	if _, err := testcore.PollState(t, blockID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	if err := boot.Ctx().CancelWorkflow(boot.Ctx(), blockID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := testcore.PollState(t, blockID, status.Cancelled); err != nil {
		t.Fatalf("poll to cancelled: %v", err)
	}

	code, body := webtest.Get(t, srv.URL+"/failed/list?page=99")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	if !strings.Contains(body, "1–3 de 3") {
		t.Errorf("pager info: %s", body)
	}
	if strings.Contains(body, "prev") || strings.Contains(body, "next") {
		t.Errorf("edge-page pager links: %s", body)
	}
}
