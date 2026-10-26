package core

import (
	"fmt"

	"flowdeck/core/graph"
)

// Spec returns the parameter spec of the named workflow (phase 5, BOOT):
// the input and the steps' default values — the web renders them on top
// of the workflow page and sanitizes the dispatch's form values against
// them before the enqueue (phase 5, EXECUTE).
func Spec(name string) (graph.Spec, error) {
	spec, ok := specOf(name)
	if !ok {
		return graph.Spec{}, fmt.Errorf("%w: %s", ErrUnknownWorkflow, name)
	}
	return spec, nil
}

// specOf is the locked lookup of a workflow's spec (no source set yet, or
// the name is not in the source).
func specOf(name string) (graph.Spec, bool) {
	theCore.mu.RLock()
	defer theCore.mu.RUnlock()
	if theCore.paths == nil {
		return graph.Spec{}, false
	}
	spec, ok := theCore.paths.Specs[name]
	return spec, ok
}
