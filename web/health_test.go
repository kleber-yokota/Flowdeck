package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"flowdeck/core"
)

// The health endpoint mirrors the core: 200 when the core is up,
// 503 when it is not.
func TestHealthz(t *testing.T) {
	up := httptest.NewServer(New(testPort(), okReady))
	defer up.Close()
	if status, _ := get(t, up.URL+"/healthz"); status != http.StatusOK {
		t.Fatalf("status: got %d want %d", status, http.StatusOK)
	}

	down := httptest.NewServer(New(testPort(), func() error { return core.ErrNotLaunched }))
	defer down.Close()
	if status, _ := get(t, down.URL+"/healthz"); status != http.StatusServiceUnavailable {
		t.Fatalf("status: got %d want %d", status, http.StatusServiceUnavailable)
	}
}
