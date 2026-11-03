package catalog

import (
	"html/template"
	"net/http"

	"flowdeck/core"
	corengraph "flowdeck/core/graph"
	"flowdeck/core/registry"
	"flowdeck/web/executions"
	"flowdeck/web/graph"
	"flowdeck/web/render"
)

// workflowPage is the data for the workflow full page: the dispatch (the
// spec form on top, the Run button for the no-input workflow), the
// executions list (what was executed, newest first) and the possible
// paths (the block diagram of the graph).
type workflowPage struct {
	Name       string
	Executions []executions.Badge
	Spec       corengraph.Spec
	Graph      template.HTML
	Title      string
}

// handleWorkflowPage renders the workflow full page: the registered name,
// the spec form (one input per spec item, defaults prefilled) and the
// possible paths (the block diagram of the graph — no graph on the
// dashboard).
func handleWorkflowPage(w http.ResponseWriter, r *http.Request, p core.Port) {
	name := r.PathValue("name")
	wf, ok := registered(name, p.List())
	if !ok {
		http.Error(w, "workflow not found", http.StatusNotFound)
		return
	}
	// The spec form: one input per spec item (the parsed source — no
	// hardcoding). The form POSTs the form values: the dispatch sanitizes
	// them against the spec before the enqueue (the web sanitizes, the
	// core trusts the caller).
	spec, err := p.Spec(name)
	if err != nil {
		render.Fail(w, "workflow spec", err)
		return
	}
	// The possible paths: the block diagram of the graph (rendered in this
	// process — map lookup, no re-parse, no DB).
	var svg template.HTML
	if block := p.Graph.Of(name); block != nil {
		svg = graph.BlockToSVG(block)
	}
	exs, err := p.Executions(name)
	if err != nil {
		render.Fail(w, "executions", err)
		return
	}
	rows := make([]executions.Badge, 0, len(exs))
	for _, e := range exs {
		rows = append(rows, executions.Row(e))
	}
	render.Page(w, "workflow.html", workflowPage{
		Name:       wf.Name,
		Executions: rows,
		Spec:       spec,
		Graph:      svg,
		Title:      "Workflow " + name,
	})
}

// registered reports whether name is in the registered list (the core
// registry, no hardcoding).
func registered(name string, ws []registry.Workflow) (registry.Workflow, bool) {
	for _, wf := range ws {
		if wf.Name == name {
			return wf, true
		}
	}
	return registry.Workflow{}, false
}
