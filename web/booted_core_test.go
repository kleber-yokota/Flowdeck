package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
)

// The booted-core tests (no stubs): the core is up from the environment
// (core.init + the workflow files' init, see .env.test), this package's
// TestMain started the executor and built the paths graph from the
// workflows tree. NewPort wraps the live core: the listing comes from the
// registry and the graph from the parsed source.
func bootedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	needCore(t)
	return httptest.NewServer(New(core.NewPort(), core.Ready))
}

// The workflow pages of a booted core (phase 1 + 4, STEP 2): the registry
// listing, the paths graph per workflow, and a dispatch that the executor
// runs to success (phase 2: the 2s poll swaps the badge).
func TestWorkflowPageWithBootedCore(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	// The core is up (the /healthz contact).
	if code, _ := get(t, srv.URL+"/healthz"); code != http.StatusOK {
		t.Fatalf("healthz: %d, want 200", code)
	}

	// The listing has every registered workflow — from the registry, no
	// hardcoding in the web.
	code, body := get(t, srv.URL+"/workflows")
	if code != http.StatusOK || !containsAll(body, ">decide<", ">hello-world<", ">pipeline<", ">buy<", ">pause<") {
		t.Fatalf("listing: %d, body: %s", code, body)
	}

	// The paths graph of each workflow (parsed source, map lookup). The
	// branched workflow draws the common prefix once and forks.
	code, decide := get(t, srv.URL+"/workflows/decide")
	if code != http.StatusOK {
		t.Fatalf("decide page: %d", code)
	}
	if !strings.Contains(decide, `<svg`) {
		t.Fatalf("decide page without the generated graph")
	}
	// The graph column only (the executions list above is out of scope).
	graphStart := strings.Index(decide, `<div class="graph">`)
	if graphStart < 0 {
		t.Fatalf("decide page without the graph column")
	}
	graph := decide[graphStart:]
	if n := strings.Count(graph, ">base<"); n != 1 {
		t.Fatalf("prefix drawn %d times, want 1", n)
	}
	for _, want := range []string{">left<", ">right<"} {
		if !strings.Contains(graph, want) {
			t.Fatalf("decide graph missing %q", want)
		}
	}
	code, pipeline := get(t, srv.URL+"/workflows/pipeline")
	if code != http.StatusOK || !containsAll(pipeline, ">fetch<", ">store<") {
		t.Fatalf("pipeline graph: %d", code)
	}

	// The schedule workflow (phase 5, DISPLAY): the form has one item per
	// input — the fields of the struct input (parsed at boot), each
	// prefilled with the default value of the source.
	code, schedule := get(t, srv.URL+"/workflows/schedule")
	if code != http.StatusOK {
		t.Fatalf("schedule page: %d", code)
	}
	for _, want := range []string{
		`name="Name"`, `name="StartDate"`,
		`Name:`, `StartDate:`,
		`value="FlowDeck"`, `value="2026-01-01"`,
	} {
		if !strings.Contains(schedule, want) {
			t.Fatalf("schedule page does not contain %q", want)
		}
	}
	// Dispatch: the struct input reaches the run (the detail shows the
	// stored input values, phase 5 DETAIL).
	resp, err := postForm(t, srv.URL+"/run/schedule", map[string][]string{
		"Name":      {"Ada"},
		"StartDate": {"2026-02-01"},
	})
	if err != nil {
		t.Fatalf("dispatch schedule: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch schedule: %d", resp.StatusCode)
	}

	// An unknown workflow is a clean 404 (the registry is the source).
	if code, _ := get(t, srv.URL+"/workflows/ghost"); code != http.StatusNotFound {
		t.Fatalf("ghost: %d, want 404", code)
	}

	// Dispatch: the ACK row is queued.
	resp, err = post(t, srv.URL+"/run/pipeline")
	if err != nil {
		t.Fatalf("dispatch: %v", err)
	}
	ack := readAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("ACK: %s", ack)
	}
	id := executionID(t, ack)

	// The executor runs the job off the queue: the badge reaches success
	// (the poller the page uses, here asserted directly).
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

// Phase 5, DETAIL: the execution detail shows the stored input values —
// the values the run used (one line per input of the struct input).
func TestExecutionDetailShowsInputValues(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := postForm(t, srv.URL+"/run/schedule", map[string][]string{
		"Name":      {"Ada"},
		"StartDate": {"2026-02-01"},
	})
	if err != nil {
		t.Fatalf("dispatch schedule: %v", err)
	}
	ack := readAll(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch schedule: %d", resp.StatusCode)
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
	code, detail := get(t, srv.URL+"/executions/"+id)
	if code != http.StatusOK {
		t.Fatalf("detail: %d", code)
	}
	for _, want := range []string{"Name", "Ada", "StartDate", "2026-02-01"} {
		if !strings.Contains(detail, want) {
			t.Fatalf("detail does not contain %q", want)
		}
	}
}
