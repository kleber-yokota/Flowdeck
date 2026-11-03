// Package webtest is the shared test scaffolding of the web packages:
// the stub Port (every core operation stubbed, the test overrides the
// one under test) and the HTTP helpers the tests use to hit the server.
package webtest

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/graph"
	"flowdeck/core/reads"
	"flowdeck/core/registry"
	"flowdeck/core/status"
)

// TestPort builds the base stub port: every core operation is a stub
// (the empty listing, the empty graph, the clean stubs below) so a test
// can override only the one operation under test.
func TestPort() core.Port {
	return core.Port{
		List:       emptyLister,
		Graph:      &graph.PathGraph{},
		Executions: okExecutions,
		Spec:       okSpec,
		Enqueue:    AckEnqueuer,
		Status:     okStatuser,
		Steps:      okSteps,
		Detail:     okDetail,
		Resume:     okResumer,
		Cancel:     okCanceller,
	}
}

func emptyLister() []registry.Workflow { return nil }

func okExecutions(name string) ([]reads.Execution, error) { return nil, nil }

// okSpec is the spec stub: a string input and one step with a default.
func okSpec(name string) (graph.Spec, error) {
	return graph.Spec{Inputs: []graph.Param{{Name: "msg", Type: "string"}}}, nil
}

// AckEnqueuer is the ACK execution stub: id + queued state + created_at.
func AckEnqueuer(name string, payload any) (reads.Execution, error) {
	return reads.Execution{
		ID:        "00000000-0000-0000-0000-000000000001",
		State:     status.Queued,
		CreatedAt: time.Date(2026, 5, 13, 13, 35, 0, 0, time.Local),
	}, nil
}

func okStatuser(workflowID string) (status.State, error) {
	return status.Queued, nil
}

// okSteps returns the executed path of a clean run: the terminal state
// (the poll stops) and a single successful step.
func okSteps(workflowID string) (reads.RunPath, error) {
	return reads.RunPath{
		State: status.Success,
		Steps: []reads.Step{{Name: "greet", State: status.Success}},
	}, nil
}

// okDetail is the detail-page stub: the unknown execution (the 404 twin of
// the booted detail tests).
func okDetail(id string) (reads.DetailPath, error) {
	return reads.DetailPath{}, reads.ErrUnknownExecution
}

func okResumer(id string) (reads.Execution, error) {
	return reads.Execution{ID: id, State: status.Queued}, nil
}

func okCanceller(id string) (reads.Execution, error) {
	return reads.Execution{ID: id, State: status.Cancelled}, nil
}

// Get issues a GET and returns the status code and the body as a string.
func Get(t *testing.T, url string) (int, string) {
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

// Post issues a POST and returns the open response (the caller reads and
// closes it).
func Post(t *testing.T, url string) (*http.Response, error) {
	t.Helper()
	return http.Post(url, "application/octet-stream", nil)
}

// PostForm posts a urlencoded form and returns the open response (the
// caller closes it).
func PostForm(t *testing.T, url string, form url.Values) (*http.Response, error) {
	t.Helper()
	return http.Post(url, "application/x-www-form-urlencoded", strings.NewReader(form.Encode()))
}

// ReadAll reads the whole response body.
func ReadAll(t *testing.T, resp *http.Response) string {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return string(body)
}

// ExecutionID extracts the workflow ID from an execution row (the dispatch
// ACK).
func ExecutionID(t *testing.T, row string) string {
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

// ContainsAll reports whether s contains every frag.
func ContainsAll(s string, frags ...string) bool {
	for _, frag := range frags {
		if !strings.Contains(s, frag) {
			return false
		}
	}
	return true
}

// ContainsAny reports whether s contains at least one frag.
func ContainsAny(s string, frags ...string) bool {
	for _, frag := range frags {
		if strings.Contains(s, frag) {
			return true
		}
	}
	return false
}
