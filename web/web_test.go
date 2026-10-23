package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flowdeck/core"
)

func emptyLister() []core.Workflow { return nil }

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

func TestRootRendersDashboardWithHTMX(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister))
	defer srv.Close()

	status, body := get(t, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{`hx-get="/workflows"`, "htmx"} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
}

// No hardcoding: the partial lists exactly what the lister (the core) returns,
// and the base page knows nothing about specific workflows.
func TestWorkflowsPartialListsCoreRegistry(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{
			{Name: "wf-alpha"},
			{Name: "wf-beta"},
		}
	}
	srv := httptest.NewServer(New(fakeLister))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{"wf-alpha", "wf-beta"} {
		if !strings.Contains(body, want) {
			t.Errorf("partial does not contain %q", want)
		}
	}

	// The full page does not hardcode workflow names
	_, page := get(t, srv.URL+"/")
	for _, hard := range []string{"wf-alpha", "wf-beta"} {
		if strings.Contains(page, hard) {
			t.Errorf("page hardcodes %q", hard)
		}
	}
}

func TestWorkflowsPartialEmptyRegistry(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, "No workflows registered") {
		t.Errorf("empty partial does not contain the message: %q", body)
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/does-not-exist")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusNotFound)
	}
}

func TestPostRootReturns405(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister))
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/", "application/octet-stream", nil)
	if err != nil {
		t.Fatalf("POST /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
