// recovery_test.go pins the recovery middleware: a handler panic becomes
// a logged 500 (the "internal error" body), not the bare
// "http: panic serving" line with no context.
package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// A panicking handler behind the recovery middleware is a 500 with the
// plain "internal error" body (the panic is logged, not crashed).
func TestRecoveryOfAPanickingHandlerIs500(t *testing.T) {
	h := withRecovery(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("recovery of a panicking handler: got %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	if got, want := rec.Body.String(), "internal error\n"; got != want {
		t.Fatalf("recovery body: got %q, want %q", got, want)
	}
}
