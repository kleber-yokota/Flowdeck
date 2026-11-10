package executions

import (
	"net/http"

	"flowdeck/core"
	"flowdeck/web/executions/badge"
	"flowdeck/web/render"
)

// handleRunDispatch is the dispatch: it reads the form (the web
// sanitizes the values against the spec), enqueues in the core and returns
// the execution row fragment (the ACK) — the browser prepends it to the
// executions list and the badge starts the 2s poll.
func handleRunDispatch(w http.ResponseWriter, r *http.Request, p core.Port) {
	name := r.PathValue("name")
	spec, err := p.Spec(name)
	if err != nil {
		render.Fail(w, "workflow spec", err)
		return
	}
	values, err := formValues(r)
	if err != nil {
		http.Error(w, "form: "+err.Error(), http.StatusBadRequest)
		return
	}
	input, err := spec.Sanitize(values)
	if err != nil {
		http.Error(w, "input: "+err.Error(), http.StatusBadRequest)
		return
	}
	ex, err := p.Enqueue(name, input)
	if err != nil {
		render.Fail(w, "enqueue", err)
		return
	}
	render.Partial(w, "execution.html", badge.Row(ex))
}

// formValues reads the form values posted to the dispatch (an empty POST
// has no form — the zero payload, the no-input dispatch).
func formValues(r *http.Request) (map[string]string, error) {
	if err := r.ParseForm(); err != nil {
		return nil, err
	}
	if len(r.Form) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(r.Form))
	for k, v := range r.Form {
		out[k] = v[0]
	}
	return out, nil
}
