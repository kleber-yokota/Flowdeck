package web

import (
	"bytes"
	"errors"
	"html/template"
	"log/slog"
	"net/http"

	"flowdeck/core"
)

// fail maps a core error onto the HTTP error response: 404 for a workflow
// or execution the core does not know, 400 for a run the core rejects
// (a run that cannot be resumed), 503 while the core is not launched, and
// a logged 500 for anything unexpected.
func fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, core.ErrUnknownWorkflow):
		http.Error(w, "unknown workflow", http.StatusNotFound)
	case errors.Is(err, core.ErrUnknownExecution):
		http.Error(w, "unknown execution", http.StatusNotFound)
	case errors.Is(err, core.ErrNotResumable):
		http.Error(w, "run cannot be resumed", http.StatusBadRequest)
	case errors.Is(err, core.ErrNotLaunched):
		http.Error(w, "core not ready", http.StatusServiceUnavailable)
	default:
		slog.Error(op, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// page renders a full HTML page (a normal navigation, so the browser may
// cache the document as usual).
func page(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(templates, w, name, data)
}

// partial renders an HTMX fragment: the content is dynamic, so it must
// never be cached by the browser.
func partial(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(templates, w, name, data)
}

// render executes the template into a buffer first, so a render error can
// never leave a partial 200 body in flight, and is always logged.
func render(t *template.Template, w http.ResponseWriter, name string, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		slog.Error("render template", "template", name, "err", err)
		http.Error(w, "error rendering", http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(buf.Bytes())
}
