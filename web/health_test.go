package web

import (
	"net/http"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/web/internal/webtest"
)

// The health endpoint mirrors the core: 200 when the core is up, 503 when
// it is not — the real boot.Ready, no stub.

func TestHealthzCoreUpIs200(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	if status, _ := webtest.Get(t, srv.URL+"/healthz"); status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}
}

func TestHealthzCoreDownIs503(t *testing.T) {
	srv := webtest.TestServer(t, New(core.NewPort(), boot.Ready))

	if status, _ := webtest.Get(t, srv.URL+"/healthz"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}
