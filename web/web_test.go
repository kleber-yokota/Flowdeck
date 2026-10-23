package web

import (
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flowdeck/core"
)

func emptyLister() []core.Workflow { return nil }

func okPing() error { return nil }

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
	srv := httptest.NewServer(New(emptyLister, okPing))
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
	srv := httptest.NewServer(New(fakeLister, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okPing))
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

func TestStaticServesVendoredHTMX(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okPing))
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

// The health endpoint mirrors the core: 200 when the core is up,
// 503 when it is not.
func TestHealthz(t *testing.T) {
	up := httptest.NewServer(New(emptyLister, okPing))
	defer up.Close()
	if status, _ := get(t, up.URL+"/healthz"); status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}

	down := httptest.NewServer(New(emptyLister, func() error { return core.ErrNotLaunched }))
	defer down.Close()
	if status, _ := get(t, down.URL+"/healthz"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// A render error must yield a clean 500 (nothing partial in flight), not a
// truncated 200 fragment injected into the HTMX DOM.
func TestRenderErrorIsClean500(t *testing.T) {
	type noName struct{ Other string }
	broken := template.Must(template.New("boom").Parse("{{ .Name }}"))

	rec := httptest.NewRecorder()
	render(broken, rec, "boom", noName{}) // .Name on a struct without it → Execute fails

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(rec.Body.String(), "error rendering") {
		t.Fatalf("body: %q", rec.Body.String())
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	srv := httptest.NewServer(New(emptyLister, okPing))
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
	srv := httptest.NewServer(New(emptyLister, okPing))
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
