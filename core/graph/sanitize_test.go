package graph

import (
	"errors"
	"testing"
)

// Phase 5, EXECUTE: the sanitization maps the raw form values onto the
// workflow's clean input — the web posts the form values, sanitizes them
// against the spec and forwards the clean input to the core. A nil values
// map is the zero payload (a dispatch without a form).

func TestSanitizeNilValuesIsZeroPayload(t *testing.T) {
	spec := Spec{Inputs: []Param{{Name: "msg", Type: "string"}}}
	payload, err := spec.Sanitize(nil)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != nil {
		t.Fatalf("payload: got %v, want nil (no form)", payload)
	}
}

func TestSanitizeStringInput(t *testing.T) {
	spec := Spec{Inputs: []Param{{Name: "msg", Type: "string"}}}
	payload, err := spec.Sanitize(map[string]string{"msg": "FlowDeck"})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != "FlowDeck" {
		t.Fatalf("payload: got %v, want \"FlowDeck\"", payload)
	}
}

func TestSanitizeIntInput(t *testing.T) {
	spec := Spec{Inputs: []Param{{Name: "n", Type: "int"}}}
	// the missing value falls back to the input's zero value
	payload, err := spec.Sanitize(map[string]string{})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != 0 {
		t.Fatalf("payload: got %v, want 0 (zero value)", payload)
	}
	// a value converts to the input's type
	payload, err = spec.Sanitize(map[string]string{"n": "42"})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != 42 {
		t.Fatalf("payload: got %v, want 42", payload)
	}
	// a value that does not convert is rejected
	_, err = spec.Sanitize(map[string]string{"n": "x"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}

// The form value is trimmed before the conversion to the input's type.
func TestSanitizeTrimsValues(t *testing.T) {
	spec := Spec{Inputs: []Param{{Name: "msg", Type: "string"}}}
	payload, err := spec.Sanitize(map[string]string{"msg": "  FlowDeck  "})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != "FlowDeck" {
		t.Fatalf("payload: got %v, want \"FlowDeck\" (trimmed)", payload)
	}
	intSpec := Spec{Inputs: []Param{{Name: "n", Type: "int"}}}
	payload, err = intSpec.Sanitize(map[string]string{"n": " 42 "})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != 42 {
		t.Fatalf("payload: got %v, want 42 (trimmed)", payload)
	}
}

// A form field that is not in the spec is rejected.
func TestSanitizeUnknownField(t *testing.T) {
	spec := Spec{Inputs: []Param{{Name: "msg", Type: "string"}}}
	_, err := spec.Sanitize(map[string]string{"msg": "x", "bogus": "y"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}

// A struct input sanitizes to the map of field → value (the clean input
// the core enqueues — the JSON object that decodes into the struct).
func TestSanitizeStructInput(t *testing.T) {
	spec := Spec{Inputs: []Param{
		{Name: "Name", Type: "string"},
		{Name: "StartDate", Type: "string"},
	}}
	// every value, trimmed, per field
	payload, err := spec.Sanitize(map[string]string{"Name": " Ada ", "StartDate": " 2026-01-01 "})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	want := map[string]any{"Name": "Ada", "StartDate": "2026-01-01"}
	if !equalPayloads(payload, want) {
		t.Fatalf("payload: got %v, want %v", payload, want)
	}
	// a missing value falls back to the field's zero value
	payload, err = spec.Sanitize(map[string]string{"Name": "Ada"})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if !equalPayloads(payload, map[string]any{"Name": "Ada", "StartDate": ""}) {
		t.Fatalf("payload: got %v, want the zero value for StartDate", payload)
	}
	// an unknown field is rejected
	_, err = spec.Sanitize(map[string]string{"Name": "Ada", "bogus": "y"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
	// a value that does not convert to a field's type is rejected
	intSpec := Spec{Inputs: []Param{{Name: "Name", Type: "string"}, {Name: "Count", Type: "int"}}}
	_, err = intSpec.Sanitize(map[string]string{"Count": "x"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}

func equalPayloads(got any, want map[string]any) bool {
	m, ok := got.(map[string]any)
	if !ok || len(m) != len(want) {
		return false
	}
	for k, v := range want {
		if m[k] != v {
			return false
		}
	}
	return true
}

// A spec without an input takes only an empty form (the zero payload).
func TestSanitizeNoInput(t *testing.T) {
	spec := Spec{}
	payload, err := spec.Sanitize(map[string]string{})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != nil {
		t.Fatalf("payload: got %v, want nil", payload)
	}
	_, err = spec.Sanitize(map[string]string{"msg": "x"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}

// An input type the sanitization does not convert is rejected (the spec
// only understands string and int).
func TestSanitizeUnsupportedType(t *testing.T) {
	spec := Spec{Inputs: []Param{{Name: "n", Type: "time.Time"}}}
	_, err := spec.Sanitize(map[string]string{"n": "1"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}
