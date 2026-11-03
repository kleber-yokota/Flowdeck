package registry

import (
	"fmt"

	"flowdeck/core/boot"
	"flowdeck/core/graph"
)

// Spec returns the parameter spec of the named workflow (phase 5, BOOT):
// the input and the steps' default values — the web renders them on top
// of the workflow page and sanitizes the dispatch's form values against
// them before the enqueue (phase 5, EXECUTE).
func Spec(name string) (graph.Spec, error) {
	spec, ok := SpecOf(name)
	if !ok {
		return graph.Spec{}, fmt.Errorf("%w: %s", ErrUnknownWorkflow, name)
	}
	return spec, nil
}

// SpecOf is the lookup of a workflow's spec (false when no source is set
// yet, or the name is not in the source) — the reads use it to render the
// stored input against the default values.
func SpecOf(name string) (graph.Spec, bool) {
	g := boot.Paths()
	if g == nil {
		return graph.Spec{}, false
	}
	spec, ok := g.Specs[name]
	return spec, ok
}
