package catalog

import (
	"net/http"
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

// The runlines of the workflow console (full page, opened from the wall)
// against the booted core: the paged execrows — the terminal row (status
// · elapsed · step · exit) with its row actions — no stubs, the rows are
// real runs of the deterministic fixtures.

// waitState polls reads.Status until the run reaches want (30s deadline);
// it reports a non-fatal failure and returns false when it does not.
func waitState(t *testing.T, id string, want status.State) bool {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		st, err := reads.Status(id)
		if err != nil {
			t.Errorf("status %s: %v", id, err)
			return false
		}
		if st == want {
			return true
		}
		if st.Terminal() {
			t.Errorf("run %s: terminal at %s, want %s", id, st, want)
			return false
		}
		if time.Now().After(deadline) {
			t.Errorf("run %s: no %s: %s", id, want, st)
			return false
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// The execrows: one row per run — the badge, the workflow id (the detail
// link), the elapsed, the step and the exit (the terminal row fields) —
// plus the table-head. The run is a real run of the deterministic
// fixtures; a terminal row has no row actions and its badge does not
// poll.
func TestWorkflowPageExecRows(t *testing.T) {
	srv := bootedTestServer(t)

	h, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "noinput-test", nil)
	if err != nil {
		t.Errorf("enqueue: %v", err)
		return
	}
	id := h.GetWorkflowID()
	if !waitState(t, id, status.Success) {
		return
	}

	code, body := webtest.Get(t, srv.URL+"/workflows/noinput-test")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	for _, want := range []string{
		`class="exec-row wf"`,
		`class="badge success terminal"`,
		`href="/executions/` + id + `"`,
		`<span class="step">noinput</span>`,
		`<span class="exit">ok</span>`,
		`executions · 1–1`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// The elapsed is the run's real duration (updated_at − created_at):
	// the run is terminal, so its times are fixed — the page must render
	// exactly the label of the same read it uses (read→render
	// consistency: no pinned value, no flake).
	p := core.NewPort()
	exs, err := p.Executions("noinput-test", execPageSize, 0)
	if err != nil {
		t.Errorf("Executions: %v", err)
		return
	}
	var ex *reads.Execution
	for i := range exs.Rows {
		if exs.Rows[i].ID == id {
			ex = &exs.Rows[i]
			break
		}
	}
	if ex == nil {
		t.Errorf("read: no row for %s", id)
		return
	}
	label := badge.ElapsedLabel(ex.UpdatedAt.Sub(ex.CreatedAt))
	if !strings.Contains(body, `<span class="elapsed">`+label+`</span>`) {
		t.Errorf("page does not render the elapsed %q: %s", label, body)
	}
	// A terminal row has no row actions (no cancel, no resume).
	if strings.Contains(body, `class="cancel"`) || strings.Contains(body, `class="resume"`) {
		t.Errorf("terminal row has row actions: %s", body)
	}
	// Terminal badge: no poll on the row.
	if strings.Contains(body, `hx-get="/status/`+id+`"`) {
		t.Errorf("terminal row badge still polls: %s", body)
	}
}

// The row actions: cancel on a live run, resume on an error run — both
// target the badge (the fragment the handler swaps). The block run
// occupies a worker until the cancel, so the cancel is registered as a
// best-effort cleanup (a cancel of an already-cancelled run is a no-op
// error, ignored).
func TestWorkflowPageTerminalRowActions(t *testing.T) {
	srv := bootedTestServer(t)

	// The live run (block mode): the row carries the cancel button and no
	// resume; the cancel takes it to Cancelled and the actions vanish.
	blockH, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "block")
	if err != nil {
		t.Errorf("enqueue block: %v", err)
		return
	}
	blockID := blockH.GetWorkflowID()
	t.Cleanup(func() {
		if c := boot.Ctx(); c != nil {
			_ = c.CancelWorkflow(c, blockID)
		}
	})
	if !waitState(t, blockID, status.Running) {
		return
	}
	code, body := webtest.Get(t, srv.URL+"/workflows/live-test")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	for _, want := range []string{
		`hx-post="/cancel/` + blockID + `"`,
		`id="badge-` + blockID + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("live row does not contain %q", want)
		}
	}
	if strings.Contains(body, `class="resume"`) {
		t.Errorf("live row has a resume button: %s", body)
	}

	// The cancel goes through the core (the cancel route is the
	// executions package's — its behavior is tested there); the console
	// asserts the row's result.
	if err := boot.Ctx().CancelWorkflow(boot.Ctx(), blockID); err != nil {
		t.Errorf("cancel: %v", err)
		return
	}

	code, body = webtest.Get(t, srv.URL+"/workflows/live-test")
	if code != http.StatusOK {
		t.Errorf("status after cancel: got %d want %d", code, http.StatusOK)
		return
	}
	if !strings.Contains(body, `class="badge cancelled terminal"`) {
		t.Errorf("page without the cancelled badge: %s", body)
	}
	if strings.Contains(body, `class="cancel"`) || strings.Contains(body, `class="resume"`) {
		t.Errorf("cancelled row has row actions: %s", body)
	}

	// The error run: the row carries the exit (the error message) and the
	// resume button, and no cancel.
	errH, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "live-test", "err")
	if err != nil {
		t.Errorf("enqueue err: %v", err)
		return
	}
	errID := errH.GetWorkflowID()
	if !waitState(t, errID, status.Error) {
		return
	}
	code, body = webtest.Get(t, srv.URL+"/workflows/live-test")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	for _, want := range []string{
		`<span class="exit">live: step failed</span>`,
		`hx-post="/resume/` + errID + `"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("error row does not contain %q", want)
		}
	}
	if strings.Contains(body, `class="cancel"`) {
		t.Errorf("error row has a cancel button: %s", body)
	}
}

// The pager: six rows per page, newest first — the older link on a full
// page, the newer link on the last page, the info (the page range).
func TestWorkflowPagePagerOlderNewer(t *testing.T) {
	srv := bootedTestServer(t)

	// The rows exist the moment the enqueue lands (the workflow row is
	// written to the system DB synchronously) — no wait needed: the
	// assertions are the counts and the links, no terminal fields.
	for i := 0; i < 7; i++ {
		if _, err := boot.Ctx().Enqueue(boot.Ctx(), boot.QueueName, "noinput-test", nil); err != nil {
			t.Errorf("enqueue %d: %v", i, err)
			return
		}
	}

	code, body := webtest.Get(t, srv.URL+"/workflows/noinput-test")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	if got := strings.Count(body, `class="exec-row`); got != 6 {
		t.Errorf("page 1: got %d execrows want 6", got)
	}
	for _, want := range []string{`href="?page=2"`, "older", "1–6"} {
		if !strings.Contains(body, want) {
			t.Errorf("page 1 does not contain %q", want)
		}
	}
	if strings.Contains(body, "newer") {
		t.Errorf("page 1 has a newer link: %s", body)
	}

	code, body = webtest.Get(t, srv.URL+"/workflows/noinput-test?page=2")
	if code != http.StatusOK {
		t.Errorf("status: got %d want %d", code, http.StatusOK)
		return
	}
	if got := strings.Count(body, `class="exec-row`); got != 1 {
		t.Errorf("page 2: got %d execrows want 1", got)
	}
	for _, want := range []string{`href="?page=1"`, "newer", "7–7"} {
		if !strings.Contains(body, want) {
			t.Errorf("page 2 does not contain %q", want)
		}
	}
	if strings.Contains(body, "older") {
		t.Errorf("page 2 has an older link: %s", body)
	}
}
