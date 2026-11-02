package core

import (
	"errors"
	"sort"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
)

// ErrUnknownWorkflow is returned when the name is not in the registry — by
// Enqueue (not registered in the DBOS) and by Spec (not in the parsed
// source).
var ErrUnknownWorkflow = errors.New("core: workflow not registered")

// Workflow is a workflow registered in the core, exposed to the web.
type Workflow struct {
	Name string
}

// List returns the workflows registered in the core DBOS — the single
// source of truth for the web listing (no hardcoding).
// It returns nil if the core is not booted (or is shutting down): the web
// then renders its "No workflows registered" empty state.
func List() []Workflow {
	c := boot.Ctx()
	if c == nil {
		return nil
	}
	entries := dbos.ListRegisteredWorkflows(c)
	out := make([]Workflow, 0, len(entries))
	for _, e := range entries {
		name := e.Name
		if name == "" {
			name = e.FQN
		}
		out = append(out, Workflow{Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
