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
	"flowdeck/core/boot"
	"flowdeck/web/internal/webtest"
)

// E2E (against the real database of the e2e job: a SQLite file — the
// booted core, the web rendering, and a dispatch that the DBOS executor
// picks off the queue and runs to success). Packages run with -p 1 so only
// one executor polls the queue at a time.
func TestE2EDispatchRunsToSuccess(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), boot.Ready))
	defer srv.Close()

	resp, err := webtest.Post(t, srv.URL+"/run/live-test")
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
}

// The dispatch posts the form values — the web sanitizes them and the run
// takes the value (the fork-test run branches to the left, the executed
// path shows it).
func TestE2EFormDispatchTakesBranch(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), boot.Ready))
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

// The resume of an id that is not known by the core is a clean 404.
func TestE2EResumeUnknownIs404(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), boot.Ready))
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

// The executed path of a run, as the execution page polls it: the steps
// appear as they execute, the poll reaches the terminal state, and the
// detail page shows the badge of the finished run.
func TestE2EExecutedPathPollsToTerminal(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), boot.Ready))
	defer srv.Close()

	resp, err := webtest.Post(t, srv.URL+"/run/linear-test")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := webtest.ReadAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("ACK: %s", ack)
	}
	id := webtest.ExecutionID(t, ack)

	// Poll the executed path partial the same way the page does, until it
	// holds the full linear path with a terminal badge.
	deadline := time.Now().Add(30 * time.Second)
	var code int
	var body string
	for {
		code, body = webtest.Get(t, srv.URL+"/executions/"+id+"/steps")
		if code == http.StatusOK &&
			webtest.ContainsAll(body, ">fetch<", ">parse<", ">transform<", ">store<") &&
			strings.Contains(body, `class="g-node success"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no terminal executed path: last code %d, body %s", code, body)
		}
		time.Sleep(500 * time.Millisecond)
	}

	// The detail page of the same run shows the finished badge.
	code, detail := webtest.Get(t, srv.URL+"/executions/"+id)
	if code != http.StatusOK || !strings.Contains(detail, `class="badge success terminal"`) {
		t.Fatalf("detail: %d, %s", code, detail)
	}
}

// The dispatch trims the form values before the enqueue (the web
// sanitizes) — the spaces-only value reaches the workflow as the zero
// value: the fork-test run branches to the right.
func TestE2ETrimsValuesBeforeDispatch(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), boot.Ready))
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
func TestE2EHTMLInInputNeverRendersRaw(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), boot.Ready))
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

// The struct input — one form item per input (the parsed spec), the web
// sanitizes the values into the map input, and the execution detail shows
// the stored input values the run used.
func TestE2EStructDispatchShowsInputValues(t *testing.T) {
	needCore(t)
	srv := httptest.NewServer(New(core.NewPort(), boot.Ready))
	defer srv.Close()

	// The form of the struct-test workflow: one item per input, prefilled
	// with the defaults of the source.
	code, page := webtest.Get(t, srv.URL+"/workflows/struct-test")
	if code != http.StatusOK {
		t.Fatalf("struct-test page: %d", code)
	}
	for _, want := range []string{`name="Name"`, `name="StartDate"`, `value="FlowDeck"`, `value="2026-01-01"`} {
		if !strings.Contains(page, want) {
			t.Fatalf("struct-test page does not contain %q", want)
		}
	}

	resp, err := webtest.PostForm(t, srv.URL+"/run/struct-test", url.Values{
		"Name":      {"Ada"},
		"StartDate": {"2026-02-01"},
	})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	ack := webtest.ReadAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch: %d", resp.StatusCode)
	}
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
	code, detail := webtest.Get(t, srv.URL+"/executions/"+id)
	if code != http.StatusOK {
		t.Fatalf("detail: %d", code)
	}
	for _, want := range []string{"Name", "Ada", "StartDate", "2026-02-01"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail does not contain %q", want)
		}
	}
}
