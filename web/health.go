package web

import (
	"net/http"
)

// handleHealthz mirrors the core: 200 when the core is ready, 503 when it
// is not (the container healthcheck uses it).
func handleHealthz(w http.ResponseWriter, ready func() error) {
	if err := ready(); err != nil {
		http.Error(w, "core not ready", http.StatusServiceUnavailable)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = w.Write([]byte("ok"))
}
