package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"flowdeck/web/internal/webtest"
)

// The form dispatch against the booted core (promoted from the e2e tag
// suite — STEP 04): the web sanitizes the form values, the run takes the
// value, and the rendered pages escape everything.

// The dispatch posts the form values — the web sanitizes them and the run
// takes the value (the fork-test run branches to the left, the executed
// path shows it).
func TestFormDispatchTakesBranch(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := webtest.PostForm(t, srv.URL+"/run/fork-test", url.Values{"msg": {"left"}})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
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
	code, steps := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Fatalf("steps: %d", code)
	}
	if !strings.Contains(steps, ">left<") || strings.Contains(steps, ">right<") {
		t.Fatalf("executed path without the left branch: %s", steps)
	}
}

// The dispatch trims the form values before the enqueue (the web
// sanitizes) — the spaces-only value reaches the workflow as the zero
// value: the fork-test run branches to the right.
func TestTrimsValuesBeforeDispatch(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := webtest.PostForm(t, srv.URL+"/run/fork-test", url.Values{"msg": {"   "}})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
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
	code, steps := webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
	if code != http.StatusOK {
		t.Fatalf("steps: %d", code)
	}
	if !strings.Contains(steps, ">right<") || strings.Contains(steps, ">left<") {
		t.Fatalf("executed path without the right branch: %s", steps)
	}
}

// A value carrying HTML dispatches cleanly and the rendered pages never
// carry the raw markup — the template layer escapes everything
// it renders.
func TestHTMLInInputNeverRendersRaw(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	const raw = `<script>alert('xss')</script>`
	resp, err := webtest.PostForm(t, srv.URL+"/run/live-test", url.Values{"mode": {raw}})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
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
	// The rendered pages (the execution detail and the workflow list) never
	// carry the raw markup.
	for _, page := range []string{"/executions/" + id, "/workflows/live-test"} {
		code, body := webtest.Get(t, srv.URL+page)
		if code != http.StatusOK {
			t.Fatalf("GET %s: %d", page, code)
		}
		if strings.Contains(body, raw) {
			t.Fatalf("page %s carries the raw markup: %s", page, body)
		}
	}
}
