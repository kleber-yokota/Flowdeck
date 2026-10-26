package web

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/graph"
	"flowdeck/core/status"

	// The workflow files register themselves in the env-driven core DBOS
	// context (their init) — the same single blank import main.go carries;
	// workflows/zz_generated.go pulls in every subpackage.
	_ "flowdeck/workflows"
)

// Shared stubs: every test file builds its httptest server from these,
// overriding only the one core operation under test.

func emptyLister() []core.Workflow { return nil }

func okExecutions(name string) ([]core.Execution, error) { return nil, nil }

// okSpec is the spec stub: a string input and one step with a default.
func okSpec(name string) (graph.Spec, error) {
	return graph.Spec{Inputs: []graph.Param{{Name: "msg", Type: "string"}}}, nil
}

// okReady is the ready stub: the core is up.
func okReady() error { return nil }

// The ACK execution of okEnqueuer: id + queued state + created_at.
func okEnqueuer(name string, payload any) (core.Execution, error) {
	return core.Execution{
		ID:        "00000000-0000-0000-0000-000000000001",
		State:     status.Queued,
		CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
	}, nil
}

func okStatuser(workflowID string) (status.State, error) {
	return status.Queued, nil
}

// okSteps returns the executed path of a clean run: a single successful
// step (phase 4, STEP 3 stub).
func okSteps(workflowID string) ([]core.Step, error) {
	return []core.Step{{Name: "greet", State: status.Success}}, nil
}

func okDetail(id string) (core.ExecutionDetail, error) {
	return core.ExecutionDetail{}, core.ErrUnknownExecution
}

// okResumer is the resume stub: the ACK is the same id, enqueued again.
func okResumer(id string) (core.Execution, error) {
	return core.Execution{ID: id, State: status.Queued}, nil
}

// testPort builds the base stub port — the shared stubs above (and the
// empty graph); each test overrides only the core operation under test.
func testPort() core.Port {
	return core.Port{
		List:       emptyLister,
		Graph:      &graph.PathGraph{},
		Executions: okExecutions,
		Spec:       okSpec,
		Enqueue:    okEnqueuer,
		Status:     okStatuser,
		Steps:      okSteps,
		Detail:     okDetail,
		Resume:     okResumer,
	}
}

// fakeGraph builds a possible-paths graph holding the given paths under
// name (the stub for the graph component in the unit tests).
func fakeGraph(name string, paths [][]string) *graph.PathGraph {
	return &graph.PathGraph{ByName: map[string][][]string{name: paths}}
}

// stateStatuser builds a statuser stub that always returns the given state.
func stateStatuser(state status.State) func(string) (status.State, error) {
	return func(workflowID string) (status.State, error) { return state, nil }
}

// get issues a GET and returns the status code and the body as a string.
func get(t *testing.T, url string) (int, string) {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, string(body)
}

// post issues a POST and returns the open response (the caller reads and
// closes it).
func post(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	return http.Post(url, "application/octet-stream", nil)
}

// readAll reads the whole response body.
func readAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// executionID extracts the workflow ID from an execution row (the dispatch
// ACK).
func executionID(t *testing.T, row string) string {
	t.Helper()
	const prefix = `href="/executions/`
	idx := strings.Index(row, prefix)
	if idx < 0 {
		t.Fatalf("row without an execution link: %q", row)
	}
	start := idx + len(prefix)
	end := start + strings.Index(row[start:], `"`)
	return row[start:end]
}

// containsAll reports whether s contains every frag.
func containsAll(s string, frags ...string) bool {
	for _, frag := range frags {
		if !strings.Contains(s, frag) {
			return false
		}
	}
	return true
}

// containsAny reports whether s contains at least one frag.
func containsAny(s string, frags ...string) bool {
	for _, frag := range frags {
		if strings.Contains(s, frag) {
			return true
		}
	}
	return false
}

// TestMain brings the env-driven core up for the booted tests: the paths
// graph from the workflows tree on disk (the unit twin of main.go's
// //go:embed workflows) and the executor (only when the env context exists
// — the mock tests run without it).
func TestMain(m *testing.M) {
	if err := core.SetWorkflowsSource(os.DirFS("../workflows")); err != nil {
		fmt.Fprintln(os.Stderr, "web: set workflows source:", err)
		os.Exit(1)
	}
	if core.Ctx() != nil {
		if err := core.Launch(); err != nil {
			fmt.Fprintln(os.Stderr, "web: launch:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// needCore skips the test when the env-driven context is absent (the env
// vars were not exported before the test binary started).
func needCore(t *testing.T) {
	t.Helper()
	if core.Ctx() == nil {
		t.Skip("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — source .env.test")
	}
}
