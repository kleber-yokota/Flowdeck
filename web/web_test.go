package web

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
)

// Shared stubs: every test file builds its httptest server from these,
// overriding only the one core operation under test.

func emptyLister() []core.Workflow { return nil }

// okPaths returns no paths (phase 4, STEP 2): the paths section is absent
// from the workflow page.
func okPaths(name string) [][]string { return nil }

func okExecutions(name string) ([]core.Execution, error) { return nil, nil }

func okPing() error { return nil }

// The ACK execution of okEnqueuer: id + queued state + created_at.
func okEnqueuer(name string, payload any) (core.Execution, error) {
	return core.Execution{
		ID:        "00000000-0000-0000-0000-000000000001",
		State:     core.StateQueued,
		CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
	}, nil
}

func okStatuser(workflowID string) (core.State, error) {
	return core.StateQueued, nil
}

// okSteps returns the executed path of a clean run: a single successful
// step (phase 4, STEP 3 stub).
func okSteps(workflowID string) ([]core.Step, error) {
	return []core.Step{{Name: "greet", State: core.StateSuccess}}, nil
}

func okDetail(id string) (core.ExecutionDetail, error) {
	return core.ExecutionDetail{}, core.ErrUnknownExecution
}

// stateStatuser builds a statuser stub that always returns the given state.
func stateStatuser(state core.State) func(string) (core.State, error) {
	return func(workflowID string) (core.State, error) { return state, nil }
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
