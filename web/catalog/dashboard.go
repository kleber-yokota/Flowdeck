package catalog

import (
	"net/http"
	"strconv"

	"flowdeck/core"
	"flowdeck/core/registry"
	"flowdeck/web/render"
)

// indexPage is the data for the index full page (the HTMX shell that loads
// the workflow list from the core registry).
type indexPage struct {
	Title string
}

// handleDashboard renders the dashboard full page (the v6.3 console: the
// HTMX shell that loads the wall and polls the live boxes).
func handleDashboard(w http.ResponseWriter) {
	render.Page(w, "index.html", indexPage{Title: "flowdeck · v6"})
}

// pageSize is the wall's page size (the spec).
const pageSize = 20

// workflowsPage is the data for the wall partial: the page of names and
// the pager (the info "start–end de total" and the prev/next links, 0 on
// the edge pages).
type workflowsPage struct {
	Items []registry.Workflow
	Start int
	End   int
	Total int
	Prev  int
	Next  int
}

// wallPage computes the wall's page from the raw ?page parameter: the page
// number clamped into [1..last] (an invalid or missing parameter is page
// 1), the slice bounds and the pager links (0 on the edge pages).
func wallPage(total int, raw string) (start, end, prev, next int) {
	page := 1
	if n, err := strconv.Atoi(raw); err == nil && n > 0 {
		page = n
	}
	last := (total + pageSize - 1) / pageSize
	if page > last {
		page = last
	}
	start = (page - 1) * pageSize
	end = start + pageSize
	if end > total {
		end = total
	}
	if page > 1 {
		prev = page - 1
	}
	if page < last {
		next = page + 1
	}
	return
}

// handleWorkflowsList renders the wall partial (the paginated dashboard
// list — from the core registry, no hardcoding; the empty state when the
// registry is empty or the core is not booted).
func handleWorkflowsList(w http.ResponseWriter, r *http.Request, p core.Port) {
	items := p.List()
	if len(items) == 0 {
		render.Partial(w, "workflows.html", workflowsPage{})
		return
	}
	start, end, prev, next := wallPage(len(items), r.URL.Query().Get("page"))
	render.Partial(w, "workflows.html", workflowsPage{
		Items: items[start:end],
		Start: start + 1,
		End:   end,
		Total: len(items),
		Prev:  prev,
		Next:  next,
	})
}

// handleLive renders the live partial (the three boxes: the Port.Counts
// counts) with no-store (the 5s poll).
func handleLive(w http.ResponseWriter, p core.Port) {
	live, err := p.Counts()
	if err != nil {
		render.Fail(w, "live", err)
		return
	}
	render.Partial(w, "live.html", live)
}
