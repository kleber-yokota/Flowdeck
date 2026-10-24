package web

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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
