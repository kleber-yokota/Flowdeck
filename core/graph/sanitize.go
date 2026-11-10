package graph

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalidInput is returned by Spec.Sanitize when the form values do not
// sanitize against the spec (an unknown field, a value that does not
// convert to the input's type). The web raises it as a 400.
var ErrInvalidInput = errors.New("graph: invalid input")

// Sanitize maps the raw form values onto the spec's clean input
// : no extra fields, each value is trimmed and converts to the
// input's type, the missing values fall back to the inputs' zero values.
// The payload is the bare value (a single simple input) or the map of
// field → value (a struct input). A nil values map is the zero payload
// (a dispatch without a form).
func (s Spec) Sanitize(values map[string]string) (any, error) {
	if values == nil {
		return nil, nil
	}
	if len(s.Inputs) == 0 {
		if len(values) != 0 {
			return nil, fmt.Errorf("%w: spec has no input", ErrInvalidInput)
		}
		return nil, nil
	}
	if err := s.checkFields(values); err != nil {
		return nil, err
	}
	if len(s.Inputs) == 1 {
		// The form value is trimmed before the conversion to the type.
		return s.Inputs[0].convert(strings.TrimSpace(values[s.Inputs[0].Name]))
	}
	payload := make(map[string]any, len(s.Inputs))
	for _, in := range s.Inputs {
		value, err := in.convert(strings.TrimSpace(values[in.Name]))
		if err != nil {
			return nil, err
		}
		payload[in.Name] = value
	}
	return payload, nil
}

// checkFields rejects a form field that is not one of the spec's inputs.
func (s Spec) checkFields(values map[string]string) error {
	known := make(map[string]struct{}, len(s.Inputs))
	for _, in := range s.Inputs {
		known[in.Name] = struct{}{}
	}
	for field := range values {
		if _, ok := known[field]; !ok {
			return fmt.Errorf("%w: unknown field %q", ErrInvalidInput, field)
		}
	}
	return nil
}

// convert maps one form value to the input's type (the spec only
// understands string and int).
func (p Param) convert(value string) (any, error) {
	switch p.Type {
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
		return nil, fmt.Errorf("%w: unsupported input type %q", ErrInvalidInput, p.Type)
	}
}
