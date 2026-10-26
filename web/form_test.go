package web

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

// Phase 5, DISPLAY: the top of the workflow page is the parameter form —
// one item per input, as key: value (the input name is the key, the
// default value of the source is the prefilled value), and the steps'
// default values (the literals of the source, read-only), from the spec
// the core parsed at boot.
func TestWorkflowPageShowsParameters(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	code, body := get(t, srv.URL+"/workflows/pipeline")
	if code != http.StatusOK {
		t.Fatalf("pipeline page: %d", code)
	}
	for _, want := range []string{
		`<form`,
		`hx-post="/run/pipeline"`,
		`name="msg"`,
		// key: value — the input name and the prefilled default
		`msg:`,
		`value="FlowDeck"`,
		// The steps' default values, read-only (the literals of the source)
		`<code>fetch</code>`,
		`<code>parse</code>`,
		`<code>transform</code>`,
		`<code>store</code>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// The form is on top: before the executions list.
	form := strings.Index(body, "<form")
	list := strings.Index(body, `id="executions"`)
	if form < 0 || list < 0 || form > list {
		t.Fatalf("form not on top: form %d, list %d", form, list)
	}
}

// Phase 5, EXECUTE: the dispatch posts the form values — the core
// sanitizes them and the run takes the value: the decide run branches to
// the left (the executed path shows it, no right).
func TestRunPostWithFormValuesTakesBranch(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := postForm(t, srv.URL+"/run/decide", url.Values{"msg": {"left"}})
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch: %d", resp.StatusCode)
	}
	ack := readAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("ACK: %s", ack)
	}
	id := executionID(t, ack)

	// The run completes taking the branch the value picked.
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

// postForm posts a urlencoded form and returns the open response (the
// caller closes it).
func postForm(t *testing.T, url string, form url.Values) (*http.Response, error) {
	t.Helper()
	return http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
}
