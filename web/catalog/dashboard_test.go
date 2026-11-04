package catalog

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/registry"
	"flowdeck/web/internal/webtest"
)

func TestRootRendersDashboardWithHTMX(t *testing.T) {
	srv := testServer(t, webtest.TestPort())
	defer srv.Close()

	status, body := webtest.Get(t, srv.URL+"/")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	// The v6.3 console: the pt-BR shell, the live poll (5s), the wall load
	// (once), the vendored htmx, the console line and the hint.
	for _, want := range []string{
		`<html lang="pt-BR">`,
		`<title>flowdeck · v6</title>`,
		`hx-get="/live"`,
		`hx-trigger="load, every 5s"`,
		`hx-get="/workflows"`,
		`<script src="/static/htmx.min.js" defer></script>`,
		`class="cmd"`,
		`class="hint"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("console does not contain %q", want)
		}
	}
}

// The live partial renders the three boxes (the counts from Port.Live,
// the pinned labels, the box hrefs) with no-store (the 5s poll).
func TestLivePartialRendersBoxes(t *testing.T) {
	d := webtest.TestPort()
	d.Live = func() (reads.Live, error) {
		return reads.Live{Running: 1, Queued: 2, Failed: 3}, nil
	}
	srv := testServer(t, d)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/live")
	if err != nil {
		t.Fatalf("GET /live: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusOK)
	}
	if got := resp.Header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control: got %q want %q", got, "no-store")
	}
	body := webtest.ReadAll(t, resp)
	for _, want := range []string{
		`href="/runnings"`,
		`href="/queued"`,
		`href="/failed"`,
		`<span class="count">1</span>`,
		`<span class="count">2</span>`,
		`<span class="count">3</span>`,
		`<span class="label">running</span>`,
		`<span class="label">queued</span>`,
		`<span class="label">cancelled · error</span>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("partial does not contain %q: %s", want, body)
		}
	}
}

// The fallback: Live returning zero counts renders the three boxes with
// 0 (the counts are data, not a failure).
func TestLivePartialFallbackZero(t *testing.T) {
	d := webtest.TestPort()
	d.Live = func() (reads.Live, error) { return reads.Live{}, nil }
	srv := testServer(t, d)
	defer srv.Close()

	status, body := webtest.Get(t, srv.URL+"/live")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if n := strings.Count(body, `<span class="count">0</span>`); n != 3 {
		t.Errorf("zero counts: got %d boxes with 0, want 3: %s", n, body)
	}
}

// The core down (not launched) maps to a 503 through the Fail map.
func TestLivePartialCoreDownIs503(t *testing.T) {
	d := webtest.TestPort()
	d.Live = func() (reads.Live, error) { return reads.Live{}, boot.ErrNotLaunched }
	srv := testServer(t, d)
	defer srv.Close()

	status, _ := webtest.Get(t, srv.URL+"/live")
	if status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}

// wallPort builds the test port of the wall tests: n registered names.
func wallPort(n int) core.Port {
	d := webtest.TestPort()
	d.List = func() []registry.Workflow {
		wfs := make([]registry.Workflow, n)
		for i := range wfs {
			wfs[i] = registry.Workflow{Name: fmt.Sprintf("wf-%02d", i)}
		}
		return wfs
	}
	return d
}

// The wall limits one page to 20 names (the spec's page size): 25
// registered workflows -> page 1 holds the first 20, not the 21st, and
// the info "1-20 de 25".
func TestWallLimitsTo20(t *testing.T) {
	srv := testServer(t, wallPort(25))
	defer srv.Close()

	status, body := webtest.Get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for i := 0; i < 20; i++ {
		if !strings.Contains(body, fmt.Sprintf("wf-%02d", i)) {
			t.Errorf("wall missing the name %d", i)
		}
	}
	if strings.Contains(body, "wf-20") {
		t.Errorf("wall holds the 21st name: %s", body)
	}
	if !strings.Contains(body, "1\u201320 de 25") {
		t.Errorf("pager info: %s", body)
	}
}

// The pager of page 2 of 2 (25 names): the info "21-25 de 25", the prev
// link to page 1, and no next link (the last page).
func TestWallPagerPrevNext(t *testing.T) {
	srv := testServer(t, wallPort(25))
	defer srv.Close()

	status, body := webtest.Get(t, srv.URL+"/workflows?page=2")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, "21\u201325 de 25") {
		t.Errorf("pager info: %s", body)
	}
	if !strings.Contains(body, `hx-get="/workflows?page=1"`) {
		t.Errorf("no prev link: %s", body)
	}
	if strings.Contains(body, `hx-get="/workflows?page=3"`) {
		t.Errorf("next link on the last page: %s", body)
	}
}

// The clamp of ?page: out-of-range pages land on the page they name —
// 99 on the last page, 0 and "abc" on page 1.
func TestWallPageClamp(t *testing.T) {
	srv := testServer(t, wallPort(25))
	defer srv.Close()

	status, body := webtest.Get(t, srv.URL+"/workflows?page=99")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, "21\u201325 de 25") {
		t.Errorf("?page=99: want the last page: %s", body)
	}
	for _, q := range []string{"page=0", "page=abc"} {
		_, body = webtest.Get(t, srv.URL+"/workflows?"+q)
		if !strings.Contains(body, "1\u201320 de 25") {
			t.Errorf("?%s: want page 1: %s", q, body)
		}
	}
}

// No hardcoding: the partial lists exactly what the lister (the core) returns,
// and the base page knows nothing about specific workflows.
func TestWorkflowsPartialListsCoreRegistry(t *testing.T) {
	fakeLister := func() []registry.Workflow {
		return []registry.Workflow{
			{Name: "wf-alpha"},
			{Name: "wf-beta"},
		}
	}
	d := webtest.TestPort()
	d.List = fakeLister
	srv := testServer(t, d)
	defer srv.Close()

	status, body := webtest.Get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	for _, want := range []string{"wf-alpha", "wf-beta"} {
		if !strings.Contains(body, want) {
			t.Errorf("partial does not contain %q", want)
		}
	}

	// The full page does not hardcode workflow names
	_, page := webtest.Get(t, srv.URL+"/")
	for _, hard := range []string{"wf-alpha", "wf-beta"} {
		if strings.Contains(page, hard) {
			t.Errorf("page hardcodes %q", hard)
		}
	}
}

func TestWorkflowsPartialEmptyRegistry(t *testing.T) {
	srv := testServer(t, webtest.TestPort())
	defer srv.Close()

	status, body := webtest.Get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, "No workflows registered") {
		t.Errorf("empty partial does not contain the message: %q", body)
	}
}

// The fragment is dynamic: it must never be cached by the browser.
func TestWorkflowsPartialNoStore(t *testing.T) {
	srv := testServer(t, webtest.TestPort())
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
	fakeLister := func() []registry.Workflow {
		return []registry.Workflow{{Name: "wf-alpha"}}
	}
	d := webtest.TestPort()
	d.List = fakeLister
	srv := testServer(t, d)
	defer srv.Close()

	status, body := webtest.Get(t, srv.URL+"/workflows")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	if !strings.Contains(body, `href="/workflows/wf-alpha"`) {
		t.Errorf("partial does not link to the workflow page: %q", body)
	}
	_, page := webtest.Get(t, srv.URL+"/")
	if strings.Contains(page, `href="/workflows/`) {
		t.Error("page hardcodes workflow links")
	}
}
