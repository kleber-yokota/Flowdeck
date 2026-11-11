package render

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// title is the data of head.html (one template, one field): the page and
// the partial both render through the shared render, so this file pins
// the two render paths (the full page the browser may cache, the partial
// it must never cache) and the successful write (nothing in flight on a
// render error — the other file).
type title struct{ Title string }

func TestPageRendersTheTemplate(t *testing.T) {
	rec := httptest.NewRecorder()
	Page(rec, "head.html", title{Title: "FlowDeck"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type: got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "<title>FlowDeck</title>") {
		t.Fatalf("body: %q", rec.Body.String())
	}
}

func TestPartialRendersWithNoStore(t *testing.T) {
	rec := httptest.NewRecorder()
	Partial(rec, "head.html", title{Title: "FlowDeck"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status: got %d want %d", rec.Code, http.StatusOK)
	}
	// The HTMX fragment is dynamic: it must never be cached by the
	// browser (the full page carries no such header).
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("Cache-Control: got %q want %q", got, "no-store")
	}
	if got := rec.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
		t.Fatalf("Content-Type: got %q", got)
	}
	if !strings.Contains(rec.Body.String(), "<title>FlowDeck</title>") {
		t.Fatalf("body: %q", rec.Body.String())
	}
}
