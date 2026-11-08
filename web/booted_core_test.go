package web

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"flowdeck/web/internal/webtest"
)

// The booted-core tests of the workflow pages (no stubs): the core is up
// from the environment (this package's TestMain env boot +
// RegisterTestWorkflows), and this package's bootedTestServer is the
// full-stack server (web.New:
// the catalog + the executions routes, the healthz, the static).

// The workflow pages of a booted core: the registry listing, the paths
// graph per workflow, and a dispatch that the executor runs to success
// (the 2s poll swaps the badge).
func TestWorkflowPageWithBootedCore(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	// The core is up (the /healthz contact).
	if code, _ := webtest.Get(t, srv.URL+"/healthz"); code != http.StatusOK {
		t.Fatalf("healthz: %d, want 200", code)
	}

	// The listing has every registered workflow — from the registry, no
	// hardcoding in the web.
	code, body := webtest.Get(t, srv.URL+"/workflows")
	if code != http.StatusOK || !webtest.ContainsAll(body, ">fork-test<", ">live-test<", ">linear-test<", ">struct-test<", ">noinput-test<") {
		t.Fatalf("listing: %d, body: %s", code, body)
	}

	// The paths graph of each workflow (parsed source, map lookup). The
	// branched workflow draws the common prefix once and forks.
	code, fork := webtest.Get(t, srv.URL+"/workflows/fork-test")
	if code != http.StatusOK {
		t.Fatalf("fork-test page: %d", code)
	}
	if !strings.Contains(fork, `<svg`) {
		t.Fatalf("fork-test page without the generated graph")
	}
	// The graph column only (the executions list above is out of scope).
	graphStart := strings.Index(fork, `<div class="graph">`)
	if graphStart < 0 {
		t.Fatalf("fork-test page without the graph column")
	}
	graph := fork[graphStart:]
	if n := strings.Count(graph, ">base<"); n != 1 {
		t.Fatalf("prefix drawn %d times, want 1", n)
	}
	for _, want := range []string{">left<", ">right<"} {
		if !strings.Contains(graph, want) {
			t.Fatalf("fork-test graph missing %q", want)
		}
	}
	code, linear := webtest.Get(t, srv.URL+"/workflows/linear-test")
	if code != http.StatusOK || !webtest.ContainsAll(linear, ">fetch<", ">store<") {
		t.Fatalf("linear-test graph: %d", code)
	}

	// The struct-test workflow: the form has one item per input — the
	// fields of the struct input (parsed at boot), each prefilled with the
	// default value of the source.
	code, structPage := webtest.Get(t, srv.URL+"/workflows/struct-test")
	if code != http.StatusOK {
		t.Fatalf("struct-test page: %d", code)
	}
	for _, want := range []string{
		`name="Name"`, `name="StartDate"`,
		`Name:`, `StartDate:`,
		`value="FlowDeck"`, `value="2026-01-01"`,
	} {
		if !strings.Contains(structPage, want) {
			t.Fatalf("struct-test page does not contain %q", want)
		}
	}
	// Dispatch: the struct input reaches the run (the detail shows the
	// stored input values).
	resp, err := webtest.PostForm(t, srv.URL+"/run/struct-test", map[string][]string{
		"Name":      {"Ada"},
		"StartDate": {"2026-02-01"},
	})
	if err != nil {
		t.Fatalf("dispatch struct-test: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch struct-test: %d", resp.StatusCode)
	}

	// An unknown workflow is a clean 404 (the registry is the source).
	if code, _ := webtest.Get(t, srv.URL+"/workflows/ghost"); code != http.StatusNotFound {
		t.Fatalf("ghost: %d, want 404", code)
	}

	// Dispatch: the ACK row is queued.
	resp, err = webtest.Post(t, srv.URL+"/run/linear-test")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := webtest.ReadAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("ACK: %s", ack)
	}
	id := webtest.ExecutionID(t, ack)

	// The executor runs the job off the queue: the badge reaches success
	// (the poller the page uses, here asserted directly).
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

// The no-input workflow (noinput-test — the blank parameter of the Go SDK)
// has no form: the page has the plain Run button, and the dispatch (an
// empty POST) enqueues with the zero payload.
func TestNoinputPageHasNoFormAndDispatches(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	code, body := webtest.Get(t, srv.URL+"/workflows/noinput-test")
	if code != http.StatusOK {
		t.Fatalf("noinput-test page: %d", code)
	}
	if strings.Contains(body, `<form`) {
		t.Fatalf("noinput-test page has a form (it has no input)")
	}
	if !strings.Contains(body, `hx-post="/run/noinput-test"`) {
		t.Fatalf("noinput-test page without the Run button")
	}

	resp, err := webtest.Post(t, srv.URL+"/run/noinput-test")
	if err != nil {
		t.Fatalf("dispatch noinput-test: %v", err)
	}
	defer resp.Body.Close()
	ack := webtest.ReadAll(t, resp)
	if resp.StatusCode != http.StatusOK || !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("dispatch noinput-test: %d, %s", resp.StatusCode, ack)
	}
}
