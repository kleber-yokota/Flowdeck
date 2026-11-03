package executions

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/web/internal/webtest"
)

// Phase 5, RESUME: a failed run shows the Resume button — the re-enqueue
// by id (the same stored input), the ACK badge replaces the badge (2s
// poll). A run that is not failed shows no button.

func TestExecutionDetailShowsResumeOnFailed(t *testing.T) {
	detailer := func(id string) (reads.DetailPath, error) {
		return reads.DetailPath{
			ExecutionDetail: reads.ExecutionDetail{
				ID:         id,
				Name:       "wf-alpha",
				State:      status.Error,
				CreatedAt:  time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
				Attempts:   1,
				Error:      "boom: step failed",
				FailedStep: "boom-step",
			},
			RunPath: reads.RunPath{State: status.Error},
		}, nil
	}
	d := webtest.TestPort()
	d.Detail = detailer
	srv := testServer(t, d)
	defer srv.Close()

	code, body := webtest.Get(t, srv.URL+"/executions/run-failed")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	for _, want := range []string{
		`hx-post="/resume/run-failed"`,
		`hx-target="#badge"`,
		"Resume",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// The detail page offers the Resume button on the cancelled page too —
// the guard accepts the cancelled row (the SDK resume re-enqueues it).
func TestExecutionDetailShowsResumeOnCancelled(t *testing.T) {
	detailer := func(id string) (reads.DetailPath, error) {
		return reads.DetailPath{
			ExecutionDetail: reads.ExecutionDetail{
				ID:        id,
				Name:      "wf-alpha",
				State:     status.Cancelled,
				CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
				Attempts:  1,
			},
			RunPath: reads.RunPath{State: status.Cancelled},
		}, nil
	}
	d := webtest.TestPort()
	d.Detail = detailer
	srv := testServer(t, d)
	defer srv.Close()

	code, body := webtest.Get(t, srv.URL+"/executions/run-cancelled")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	for _, want := range []string{
		`hx-post="/resume/run-cancelled"`,
		`hx-target="#badge"`,
		"Resume",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

func TestExecutionDetailNoResumeOnSuccess(t *testing.T) {
	detailer := func(id string) (reads.DetailPath, error) {
		return reads.DetailPath{
			ExecutionDetail: reads.ExecutionDetail{
				ID:        id,
				Name:      "wf-alpha",
				State:     status.Success,
				CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
				Attempts:  1,
			},
			RunPath: reads.RunPath{State: status.Success},
		}, nil
	}
	d := webtest.TestPort()
	d.Detail = detailer
	srv := testServer(t, d)
	defer srv.Close()

	code, body := webtest.Get(t, srv.URL+"/executions/run-ok")
	if code != http.StatusOK {
		t.Fatalf("status: got %d want %d", code, http.StatusOK)
	}
	if strings.Contains(body, "/resume/") {
		t.Fatalf("success page must not offer a resume: %s", body)
	}
}

// The resume of an id that is not known by the core is a clean 404 (the
// core maps it to ErrUnknownExecution).
func TestResumeUnknownIs404(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := webtest.Post(t, srv.URL+"/resume/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("POST /resume: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("resume unknown: got %d, want 404", resp.StatusCode)
	}
}

// The resume of a cancelled run (booted): the dispatch, the cancel, and
// the resume — the ACK badge is queued with the same id (the 2s poll
// takes over from the badge).
func TestResumeBootedRunRendersQueuedBadge(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := webtest.Post(t, srv.URL+"/run/pause")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := webtest.ReadAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("dispatch ACK: %s", ack)
	}
	id := webtest.ExecutionID(t, ack)

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
}

// The resume of a run that is not failed (here: success) is a clean 400
// (the core maps it to ErrNotResumable).
func TestResumeOfSuccessIs400(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := webtest.Post(t, srv.URL+"/run/hello-world")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := webtest.ReadAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("ACK: %s", ack)
	}
	id := webtest.ExecutionID(t, ack)

	deadline := time.Now().Add(30 * time.Second)
	for {
		code, row := webtest.Get(t, srv.URL+"/executions/"+id)
		if code == http.StatusOK && strings.Contains(row, `class="badge success terminal"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no success badge: %d, %s", code, row)
		}
		time.Sleep(500 * time.Millisecond)
	}

	resume, err := webtest.Post(t, srv.URL+"/resume/"+id)
	if err != nil {
		t.Fatalf("POST /resume: %v", err)
	}
	resume.Body.Close()
	if resume.StatusCode != http.StatusBadRequest {
		t.Fatalf("resume of success: got %d, want 400", resume.StatusCode)
	}
}
