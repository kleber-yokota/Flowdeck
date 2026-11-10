package catalog

import (
	"html/template"
	"net/http"
	"path"
	"strconv"

	"flowdeck/core"
	corengraph "flowdeck/core/graph"
	"flowdeck/core/reads"
	"flowdeck/core/registry"
	"flowdeck/core/status"
	"flowdeck/web/executions/badge"
	blocksvg "flowdeck/web/graph/block"
	"flowdeck/web/render"
)

// execPageSize is the console's runline page size (six per page).
const execPageSize = 6

// execPager computes the pager figures of the console runline listing:
// the range shown (start–end), the prev link (only when there is a page
// before this one) and the next link (only when there are more rows).
func execPager(page, count int, hasMore bool) (start, end, prev, next int) {
	if page < 1 {
		page = 1
	}
	start = (page-1)*execPageSize + 1
	end = start + count - 1
	prev = page - 1
	next = 0
	if hasMore {
		next = page + 1
	}
	return
}

// workflowPage is the data for the workflow console: the cmd line with
// the source breadcrumb, the runline dispatch (the spec form below the
// dispatch line), the paged runlines (six per page, newest first, with
// the terminal row fields and the row actions) and the graph side (the
// block diagram of the possible paths, with the legend).
type workflowPage struct {
	Name     string
	Rows     []badge.Runline
	Count    int
	Start    int
	End      int
	Prev     int
	Next     int
	Spec     corengraph.Spec
	Graph    template.HTML
	FileDir  string
	FileBase string
	Title    string
}

// handleWorkflowPage renders the workflow console: the registered name,
// the cmd line with the source breadcrumb, the runline dispatch (one
// input per spec item, defaults prefilled — the zero payload for the
// no-input workflow), the paged runlines (the terminal rows enriched
// with the last step and the exit) and the possible paths (the block
// diagram of the graph — no graph on the dashboard).
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
		svg = blocksvg.BlockToSVG(block)
	}
	// The source breadcrumb: the workflow's source file from the boot map
	// (the file that declared the workflow func — map lookup, no re-parse,
	// no scan). A flat file (dir ".") renders without the dir span.
	file := p.Graph.Files[name]
	fileDir := path.Dir(file)
	if fileDir == "." {
		fileDir = ""
	}
	fileBase := path.Base(file)

	// The paged runlines: parse ?page (clamped to 1), the offset is
	// (page-1) times the page size.
	page := pageFrom(r)
	exs, err := p.Executions(name, execPageSize, (page-1)*execPageSize)
	if err != nil {
		render.Fail(w, "executions", err)
		return
	}
	rows := make([]badge.Runline, 0, len(exs.Rows))
	for _, e := range exs.Rows {
		row := badge.Row(e)
		if row.Terminal {
			terminalRowFields(p, e, &row)
		}
		rows = append(rows, row)
	}
	start, end, prev, next := execPager(page, len(rows), exs.HasMore)
	render.Page(w, "workflow.html", workflowPage{
		Name:     wf.Name,
		Rows:     rows,
		Count:    len(rows),
		Start:    start,
		End:      end,
		Prev:     prev,
		Next:     next,
		Spec:     spec,
		Graph:    svg,
		FileDir:  fileDir,
		FileBase: fileBase,
		Title:    "Workflow " + name,
	})
}

// pageFrom parses ?page (clamped to 1).
func pageFrom(r *http.Request) int {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		return 1
	}
	return page
}

// terminalRowFields enriches the terminal runline: the elapsed (the status
// update time minus the created), the last step of the executed path (a
// failed steps read leaves the row unenriched) and the exit — the decoded
// output on success, the error message on error (facts F2 · F3 · F4).
func terminalRowFields(p core.Port, e reads.Execution, row *badge.Runline) {
	row.Elapsed = badge.ElapsedLabel(e.UpdatedAt.Sub(e.CreatedAt))
	if rp, err := p.Steps(e.ID); err == nil && len(rp.Steps) > 0 {
		row.Step = rp.Steps[len(rp.Steps)-1].Name
	}
	switch e.State {
	case status.Success:
		row.Exit = e.Output
	case status.Error:
		row.Exit = e.Error
	}
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
