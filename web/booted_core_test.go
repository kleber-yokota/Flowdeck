package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flowdeck/core"

	_ "github.com/dbos-inc/dbos-transact-golang/dbos/driver/sqlite"
)

// bootSQLiteCoreForTest resets the core singleton and boots a fresh core
// on a temporary SQLite database (the DBOS sqlite driver) — no Postgres
// needed for the full-stack test, the same fast cycle the core tests use.
func bootSQLiteCoreForTest(t *testing.T) {
	t.Helper()

	core.Shutdown(5 * time.Second) // nil-safe: this test starts clean
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	if err := core.Boot(ctx, "flowdeck_web_test", "sqlite:"+filepath.Join(t.TempDir(), "web-test.db")); err != nil {
		t.Fatalf("boot: %v", err)
	}
	t.Cleanup(func() { core.Shutdown(5 * time.Second) })
	if err := core.Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}
}

// TestWorkflowPageWithBootedCore: the full stack with a booted core — the
// paths graph on the page comes from the real boot map (phase 4, STEP 2:
// map lookup, no re-parse, no DB), and a dispatch runs to success.
func TestWorkflowPageWithBootedCore(t *testing.T) {
	bootSQLiteCoreForTest(t)

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

	// Dispatch (phase 2): the web enqueues on the core; the executor runs
	// the run to success (the badge is terminal: "badge success terminal").
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

	deadline := time.Now().Add(30 * time.Second)
	for {
		if _, body := get(t, srv.URL+"/status/"+id); strings.Contains(body, `class="badge success terminal"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run %s did not reach success in time", id)
		}
		time.Sleep(200 * time.Millisecond)
	}
}
