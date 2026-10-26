package core

import (
	"errors"
	"fmt"
	"strconv"

	"flowdeck/core/graph"
)

// ErrInvalidInput is returned by Enqueue when the raw form values do not
// sanitize against the workflow's spec (an unknown field, a value that does
// not convert to the input's type). The web maps it to a 400.
var ErrInvalidInput = errors.New("core: invalid input")

// Spec returns the parameter spec of the named workflow (phase 5, BOOT):
// the input and the steps' default values — the web renders them on top
// of the workflow page and the dispatch sanitizes the form values against
// them.
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

// sanitize maps the raw form values onto the workflow's clean input
// (phase 5, EXECUTE): no extra fields, the missing value falls back to
// the input's zero value, and the value converts to the input's type. A
// nil values map is the zero payload (a dispatch without a form).
func sanitize(name string, spec graph.Spec, values map[string]string) (any, error) {
	if values == nil {
		return nil, nil
	}
	if spec.Input.Name == "" {
		if len(values) != 0 {
			return nil, fmt.Errorf("%w: %s has no input", ErrInvalidInput, name)
		}
		return nil, nil
	}
	for field := range values {
		if field != spec.Input.Name {
			return nil, fmt.Errorf("%w: unknown field %q", ErrInvalidInput, field)
		}
	}
	return convertInput(spec.Input.Type, values[spec.Input.Name])
}

// convertInput converts one form value to the input's type (the spec only
// understands string and int).
func convertInput(inputType, value string) (any, error) {
	switch inputType {
	case "string":
		return value, nil
	case "int":
		if value == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(value)
		if err != nil {
			return nil, fmt.Errorf("%w: %q is not an int", ErrInvalidInput, value)
		}
		return n, nil
	default:
		return nil, fmt.Errorf("%w: unsupported input type %q", ErrInvalidInput, inputType)
	}
}
