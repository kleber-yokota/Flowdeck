package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flowdeck/core"
)

func TestRootRendersDashboardWithHTMX(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	// The HTMX shell: the vendored script tag and the HTMX trigger that
	// loads the partial from the core registry.
	for _, want := range []string{
		`hx-get="/workflows"`,
		`<script src="/static/htmx.min.js" defer></script>`,
	} {
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
	srv := httptest.NewServer(New(fakeLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, "No workflows registered") {
		t.Errorf("empty partial does not contain the message: %q", body)
	}
}

// The fragment is dynamic: it must never be cached by the browser.
func TestWorkflowsPartialNoStore(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/workflows")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if _, err := io.Copy(io.Discard, resp.Body); err != nil {
		t.Fatalf("read body: %v", err)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
}

// The partial links each registered workflow to its page (zero hardcoding
// in the base page): the dispatch button lives on that page.
func TestWorkflowsPartialLinksToWorkflowPages(t *testing.T) {
	fakeLister := func() []core.Workflow {
		return []core.Workflow{{Name: "wf-alpha"}}
	}
	srv := httptest.NewServer(New(fakeLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, `href="/workflows/wf-alpha"`) {
		t.Errorf("partial does not link to the workflow page: %q", body)
	}
	_, page := get(t, srv.URL+"/")
	if strings.Contains(page, `href="/workflows/`) {
		t.Error("page hardcodes workflow links")
	}
}

// The vendored assets are served from the embedded FS — no CDN at runtime.
func TestStaticServesVendoredHTMX(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
	defer srv.Close()

	status, body := get(t, srv.URL+"/static/htmx.min.js")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	// The vendored bundle declares its own version
	if !strings.Contains(body, `version:"2.0.4"`) {
		t.Error("vendored htmx does not declare version 2.0.4")
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okPaths, okExecutions, okEnqueuer, okStatuser, okSteps, okDetail, okPing))
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
