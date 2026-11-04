package executions

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"flowdeck/core/boot"
	"flowdeck/core/status"
	"flowdeck/web/internal/webtest"
)

// A failed run shows the Resume button — the re-enqueue by id (the same
// stored input), the ACK badge replaces the badge (2s poll). A run that is
// not failed shows no button. Real runs (the booted core, no stubs).

// The detail page offers the Resume button (hx-post + hx-target=#badge)
// on the failed and the cancelled runs — the guard accepts the cancelled
// row too (the SDK resume re-enqueues it) — and no button on the success
// run.
func TestExecutionDetailResumeButton(t *testing.T) {
	for _, tc := range []struct {
		st   status.State
		want bool
	}{
		{status.Error, true},
		{status.Cancelled, true},
		{status.Success, false},
	} {
		t.Run(string(tc.st), func(t *testing.T) {
			srv := bootedTestServer(t)

			id := runInState(t, srv, tc.st)
			code, body := webtest.Get(t, srv.URL+"/executions/"+id)
			if code != http.StatusOK {
				t.Fatalf("detail: %d", code)
			}
			has := strings.Contains(body, `hx-post="/resume/`+id+`"`) && strings.Contains(body, `hx-target="#badge"`)
			if has != tc.want {
				t.Errorf("resume button: got %v want %v: %s", has, tc.want, body)
			}
			if tc.want && !strings.Contains(body, "Resume") {
				t.Errorf("page without the Resume label: %s", body)
			}
		})
	}
}

// The resume of an id that is not known by the core is a clean 404 (the
// core maps it to ErrUnknownExecution).
func TestResumeUnknownIs404(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := webtest.Post(t, srv.URL+"/resume/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("POST /resume: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("resume unknown: got %d, want 404", resp.StatusCode)
	}
}

// The resume of a cancelled run (booted): the dispatch, the cancel, and
// the resume — the ACK badge is queued with the same id (the 2s poll takes
// over from the badge). The block-test run blocks until the cancel, and
// the resumed run blocks again (the block step has no recorded result at
// the cancel) — the final cancel unblocks it.
func TestResumeBootedRunRendersQueuedBadge(t *testing.T) {
	srv := bootedTestServer(t)

	resp, err := webtest.Post(t, srv.URL+"/run/block-test")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := webtest.ReadAll(t, resp)
	if resp.StatusCode != http.StatusOK || !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("dispatch ACK: %d, %s", resp.StatusCode, ack)
	}
	id := webtest.ExecutionID(t, ack)
	t.Cleanup(func() {
		if c := boot.Ctx(); c != nil {
			_ = c.CancelWorkflow(c, id)
		}
	})

	// The run is live (running): poll the detail badge until the running
	// badge (the deterministic Running state — no timing window).
	deadline := time.Now().Add(30 * time.Second)
	for {
		code, body := webtest.Get(t, srv.URL+"/executions/"+id)
		if code == http.StatusOK && strings.Contains(body, `class="badge running"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no running badge: %d, %s", code, body)
		}
		time.Sleep(200 * time.Millisecond)
	}

	cancel, err := webtest.Post(t, srv.URL+"/cancel/"+id)
	if err != nil {
		t.Fatalf("cancel: %v", err)
	}
	cack := webtest.ReadAll(t, cancel)
	if cancel.StatusCode != http.StatusOK || !strings.Contains(cack, `class="badge cancelled terminal"`) {
		t.Fatalf("cancel ACK: %d, %s", cancel.StatusCode, cack)
	}

	resume, err := webtest.Post(t, srv.URL+"/resume/"+id)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	rack := webtest.ReadAll(t, resume)
	if resume.StatusCode != http.StatusOK || !strings.Contains(rack, `class="badge queued"`) {
		t.Fatalf("resume ACK: %d, %s", resume.StatusCode, rack)
	}
	if !strings.Contains(rack, `data-workflow-id="`+id+`"`) {
		t.Fatalf("resume ACK without the same id: %s", rack)
	}

	// The resumed block run blocks again — the cancel unblocks the step
	// (the suite settles).
	cleanup, err := webtest.Post(t, srv.URL+"/cancel/"+id)
	if err != nil {
		t.Fatalf("cleanup cancel: %v", err)
	}
	cleanup.Body.Close()
	if cleanup.StatusCode != http.StatusOK {
		t.Fatalf("cleanup cancel: %d", cleanup.StatusCode)
	}
}

// The resume of a run that is not failed (here: success) is a clean 400
// (the core maps it to ErrNotResumable).
func TestResumeOfSuccessIs400(t *testing.T) {
	srv := bootedTestServer(t)

	ack := webtest.Ack(t, srv.URL, "live-test", nil)
	id := webtest.ExecutionID(t, ack)
	webtest.Detail(t, srv.URL, id, `class="badge success terminal"`)

	resp, err := webtest.Post(t, srv.URL+"/resume/"+id)
	if err != nil {
		t.Fatalf("POST /resume: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("resume of success: got %d, want 400", resp.StatusCode)
	}
}
