package web

import (
	"io"
	"net/http"
	"testing"
	"time"

	"flowdeck/core"
)

// Shared stubs: every test file builds its httptest server from these,
// overriding only the one core operation under test.

func emptyLister() []core.Workflow { return nil }

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
