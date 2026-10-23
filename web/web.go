package web

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"

	"flowdeck/core"
)

//go:embed templates/index.html templates/workflows.html
var templatesFS embed.FS

// New builds the HTMX web server — an abstraction over the core.
// listWorkflows returns the workflows registered in the core (single
// source of truth); the web knows nothing about specific workflows.
func New(listWorkflows func() []core.Workflow) http.Handler {
	page, err := template.ParseFS(templatesFS, "templates/index.html")
	if err != nil {
		panic(fmt.Sprintf("template index: %v", err))
	}
	partial, err := template.ParseFS(templatesFS, "templates/workflows.html")
	if err != nil {
		panic(fmt.Sprintf("template workflows: %v", err))
	}

	mux := http.NewServeMux()

	// Full page (serves the HTMX shell)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := page.Execute(w, nil); err != nil {
			http.Error(w, "error rendering page", http.StatusInternalServerError)
		}
	})

	// HTMX partial: fragment with the workflows registered in the core
	mux.HandleFunc("GET /workflows", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := partial.Execute(w, listWorkflows()); err != nil {
			http.Error(w, "error rendering workflows", http.StatusInternalServerError)
		}
	})

	return mux
}
