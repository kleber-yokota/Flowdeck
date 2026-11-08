package web

import (
	"net/http"
	"strings"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/web/internal/webtest"
)

// The routing layer (no core state involved): the static assets, the 404
// and the 405 — the real port, the real ready check.

// The vendored assets are served from the embedded FS — no CDN at runtime.
func TestStaticServesVendoredHTMX(t *testing.T) {
	srv := webtest.TestServer(t, New(core.NewPort(), boot.Ready))

	status, body := webtest.Get(t, srv.URL+"/static/htmx.min.js")
	if status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
	// The vendored bundle declares its own version
	if !strings.Contains(body, `version:"2.0.4"`) {
		t.Error("vendored htmx does not declare version 2.0.4")
	}
}

func TestUnknownPathReturns404(t *testing.T) {
	srv := webtest.TestServer(t, New(core.NewPort(), boot.Ready))

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
	srv := webtest.TestServer(t, New(core.NewPort(), boot.Ready))

	resp, err := http.Post(srv.URL+"/", "application/octet-stream", nil)
	if err != nil {
		t.Fatalf("POST /: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("status: got %d want %d", resp.StatusCode, http.StatusMethodNotAllowed)
	}
}
