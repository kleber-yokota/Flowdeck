//go:build e2e

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
)

// E2E (against the real Postgres 18 of the e2e job): the full stack on the
// production database engine — the env-driven core, the web rendering, and
// a dispatch that the DBOS executor picks off the queue and runs to
// success. Packages run with -p 1 so only one executor polls the queue at a
// time.
func TestE2EDispatchRunsToSuccess(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), core.Ready))
	defer srv.Close()

	resp, err := post(t, srv.URL+"/run/hello-world")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := readAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("ACK: %s", ack)
	}
	id := executionID(t, ack)

	deadline := time.Now().Add(30 * time.Second)
	for {
		code, row := get(t, srv.URL+"/executions/"+id)
		if code == http.StatusOK && strings.Contains(row, `class="badge success terminal"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no success badge: %d, %s", code, row)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Phase 5, E2E: the dispatch posts the form values — the core sanitizes
// them and the run takes the value on Postgres (the decide run branches to
// the left, the executed path shows it).
func TestE2EFormDispatchTakesBranch(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), core.Ready))
	defer srv.Close()

	resp, err := postForm(t, srv.URL+"/run/decide", url.Values{"msg": {"left"}})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	ack := readAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("ACK: %s", ack)
	}
	id := executionID(t, ack)

	deadline := time.Now().Add(30 * time.Second)
	for {
		code, row := get(t, srv.URL+"/executions/"+id)
		if code == http.StatusOK && strings.Contains(row, `class="badge success terminal"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no success badge: %d, %s", code, row)
		}
		time.Sleep(500 * time.Millisecond)
	}
	code, steps := get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Fatalf("steps: %d", code)
	}
	if !strings.Contains(steps, ">left<") || strings.Contains(steps, ">right<") {
		t.Fatalf("executed path without the left branch: %s", steps)
	}
}

// Phase 5, RESUME, E2E: the resume of an id that is not known by the core
// is a clean 404 on Postgres.
func TestE2EResumeUnknownIs404(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), core.Ready))
	defer srv.Close()

	resp, err := post(t, srv.URL+"/resume/00000000-0000-0000-0000-000000000000")
	if err != nil {
		t.Fatalf("POST /resume: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("resume unknown: got %d, want 404", resp.StatusCode)
	}
}

// The executed path of a run, as the execution page polls it (phase 4,
// STEP 3, E2E): the steps appear as they execute, the poll reaches the
// terminal state, and the detail page shows the badge of the finished run.
func TestE2EExecutedPathPollsToTerminal(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), core.Ready))
	defer srv.Close()

	resp, err := post(t, srv.URL+"/run/pipeline")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := readAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("ACK: %s", ack)
	}
	id := executionID(t, ack)

	// Poll the executed path partial the same way the page does, until it
	// holds the full linear path with a terminal badge.
	deadline := time.Now().Add(30 * time.Second)
	var code int
	var body string
	for {
		code, body = get(t, srv.URL+"/executions/"+id+"/steps")
		if code == http.StatusOK &&
			containsAll(body, ">fetch<", ">parse<", ">transform<", ">store<") &&
			strings.Contains(body, `class="g-node success"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no terminal executed path: last code %d, body %s", code, body)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// The detail page of the same run shows the finished badge.
	code, detail := get(t, srv.URL+"/executions/"+id)
	if code != http.StatusOK || !strings.Contains(detail, `class="badge success terminal"`) {
		t.Fatalf("detail: %d, %s", code, detail)
	}
}
