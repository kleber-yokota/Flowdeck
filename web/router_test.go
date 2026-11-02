package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The vendored assets are served from the embedded FS — no CDN at runtime.
func TestStaticServesVendoredHTMX(t *testing.T) {
	srv := httptest.NewServer(New(testPort(), okReady))
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
	srv := httptest.NewServer(New(testPort(), okReady))
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
	srv := httptest.NewServer(New(testPort(), okReady))
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
