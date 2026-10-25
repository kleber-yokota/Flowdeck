//go:build e2e

package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
)

// End-to-end (against a real Postgres 18): the full stack — the core boots
// and the web renders the workflow pages with the paths graph from the
// boot map (phase 4, STEP 2: map lookup, no re-parse, no DB), and a
// dispatch runs to success.
func TestE2EWorkflowPageGraph(t *testing.T) {
	dbURL := os.Getenv("DBOS_E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("DBOS_E2E_DATABASE_URL not set — skipping e2e test")
	}

	core.Shutdown(5 * time.Second) // nil-safe: this test starts clean
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	// A dedicated system database (DBOS auto-creates it): the core e2e and
	// this test may boot concurrently, and queue names must be unique per
	// system database.
	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatalf("parse DBOS_E2E_DATABASE_URL: %v", err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "_web"
	u.RawPath = ""
	if err := core.Boot(ctx, "flowdeck_web_e2e", u.String()); err != nil {
		t.Fatalf("boot: %v", err)
	}
	t.Cleanup(func() { core.Shutdown(5 * time.Second) })
	if err := core.Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	srv := httptest.NewServer(New(core.NewPort(), core.Ready))
	defer srv.Close()

	// The list comes from the core registry (never hardcoded in the web).
	if status, body := get(t, srv.URL+"/workflows"); status != http.StatusOK ||
		!strings.Contains(body, "decide") || !strings.Contains(body, "pipeline") || !strings.Contains(body, "hello-world") {
		t.Fatalf("workflows list: status %d, body %q", status, body)
	}

	// The generated graph (phase 4, STEP 2): the if forks the branches —
	// the common prefix is drawn once.
	status, body := get(t, srv.URL+"/workflows/decide")
	if status != http.StatusOK {
		t.Fatalf("decide page: status %d, want 200", status)
	}
	if !strings.Contains(body, `<svg`) {
		t.Error("decide page without the generated graph")
	}
	if n := strings.Count(body, ">base<"); n != 1 {
		t.Errorf("prefix drawn %d times, want 1", n)
	}
	for _, want := range []string{">left<", ">right<"} {
		if !strings.Contains(body, want) {
			t.Errorf("decide graph missing %q", want)
		}
	}

	// The linear workflow: one row of steps.
	if status, body = get(t, srv.URL+"/workflows/pipeline"); status != http.StatusOK ||
		!strings.Contains(body, ">fetch<") || !strings.Contains(body, ">store<") {
		t.Fatalf("pipeline page: status %d", status)
	}

	// An unknown workflow is a clean 404.
	if status, _ := get(t, srv.URL+"/workflows/ghost"); status != http.StatusNotFound {
		t.Errorf("ghost: status %d, want 404", status)
	}

	// Dispatch (phase 2): the web enqueues on the core; the DBOS executor
	// runs the run to success.
	resp, err := post(t, srv.URL+"/run/pipeline")
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch: status %d, want 200", resp.StatusCode)
	}
	ack := readAll(t, resp)
	if !strings.Contains(ack, `class="badge queued"`) {
		t.Fatalf("dispatch ACK without the queued badge: %q", ack)
	}
	id := executionID(t, ack)

	// Poll the badge partial until the run reaches its terminal state
	// (success is always terminal: "badge success terminal").
	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, body := get(t, srv.URL+"/status/"+id); strings.Contains(body, `class="badge success terminal"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not reach success in time", id)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// The execution page shows the executed path (phase 4, STEP 3): the steps
// partial is polled every 2s and stops when the run is terminal — the
// fragment then shows the path that ran, as the same SVG graph as the
// possible paths (read from Postgres, separate from the boot map).
func TestE2EExecutedPathPollsToTerminal(t *testing.T) {
	dbURL := os.Getenv("DBOS_E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("DBOS_E2E_DATABASE_URL not set — skipping e2e test")
	}

	core.Shutdown(5 * time.Second) // nil-safe: this test starts clean
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	// The dedicated system database (the "_web" suffix) isolates this test
	// from the core e2e boots that may run concurrently.
	u, err := url.Parse(dbURL)
	if err != nil {
		t.Fatalf("parse DBOS_E2E_DATABASE_URL: %v", err)
	}
	u.Path = strings.TrimSuffix(u.Path, "/") + "_web_steps"
	u.RawPath = ""
	if err := core.Boot(ctx, "flowdeck_web_steps_e2e", u.String()); err != nil {
		t.Fatalf("boot: %v", err)
	}
	t.Cleanup(func() { core.Shutdown(5 * time.Second) })
	if err := core.Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	srv := httptest.NewServer(New(core.NewPort(), core.Ready))
	defer srv.Close()

	// Dispatch a linear run: the executed path is fetch → parse → transform
	// → store.
	resp, err := post(t, srv.URL+"/run/pipeline")
	if err != nil {
		t.Fatalf("POST /run: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("dispatch: status %d, want 200", resp.StatusCode)
	}
	id := executionID(t, readAll(t, resp))

	// Poll the steps partial (the page's 2s poll) until the run is
	// terminal: the fragment then has no hx attributes.
	deadline := time.Now().Add(30 * time.Second)
	var body string
	for {
		if _, body = get(t, srv.URL+"/executions/"+id+"/steps"); !strings.Contains(body, `hx-trigger="every 2s"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not reach a terminal state in time", id)
		}
		time.Sleep(500 * time.Millisecond)
	}
	for _, want := range []string{`<svg`, `>fetch<`, `>parse<`, `>transform<`, `>store<`} {
		if !strings.Contains(body, want) {
			t.Errorf("terminal fragment missing %q", want)
		}
	}

	// The full page embeds the executed path (the same SVG).
	if status, body := get(t, srv.URL+"/executions/"+id); status != http.StatusOK ||
		!strings.Contains(body, `>fetch<`) || !strings.Contains(body, `>store<`) {
		t.Fatalf("execution page: status %d, body %q", status, body)
	}
}
