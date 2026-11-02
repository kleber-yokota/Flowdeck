// Package render is the rendering of FlowDeck's web: the templates
// (embedded — one named template per file, rendered per request, no
// template cache), the two render paths (the full page, which the browser
// may cache, and the HTMX partial, which it must never cache), and the
// mapping of the core's errors onto the HTTP error responses.
package render

import (
	"bytes"
	"embed"
	"errors"

	"html/template"
	"io/fs"
	"log/slog"
	"net/http"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/registry"
)

// templates is the template set (templates/, one named template per file).
var templates *template.Template

//
//go:embed templates
var templatesFS embed.FS

func init() {
	entries, err := fs.ReadDir(templatesFS, "templates")
	if err != nil {
		panic("render: templates: " + err.Error())
	}
	templates = template.New("web")
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		b, err := templatesFS.ReadFile("templates/" + e.Name())
		if err != nil {
			panic("render: templates/" + e.Name() + ": " + err.Error())
		}
		template.Must(templates.New(e.Name()).Parse(string(b)))
	}
}

// Fail maps a core error onto the HTTP error response: 404 for a workflow
// or execution the core does not know, 400 for a run the core rejects
// (a run that cannot be resumed or cancelled), 503 while the core is not
// launched, and a logged 500 for anything unexpected.
func Fail(w http.ResponseWriter, op string, err error) {
	switch {
	case errors.Is(err, registry.ErrUnknownWorkflow):
		http.Error(w, "unknown workflow", http.StatusNotFound)
	case errors.Is(err, reads.ErrUnknownExecution):
		http.Error(w, "unknown execution", http.StatusNotFound)
	case errors.Is(err, actions.ErrNotResumable):
		http.Error(w, "run cannot be resumed", http.StatusBadRequest)
	case errors.Is(err, actions.ErrNotCancellable):
		http.Error(w, "run cannot be cancelled", http.StatusBadRequest)
	case errors.Is(err, boot.ErrNotLaunched):
		http.Error(w, "core not ready", http.StatusServiceUnavailable)
	default:
		slog.Error(op, "err", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
	}
}

// Page renders a full HTML page (a normal navigation, so the browser may
// cache the document as usual).
func Page(w http.ResponseWriter, name string, data any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	render(templates, w, name, data)
}

// Partial renders an HTMX fragment: the content is dynamic, so it must
// never be cached by the browser.
func Partial(w http.ResponseWriter, name string, data any) {
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
