package core

import (
	"errors"
	"testing"

	"flowdeck/core/graph"
)

// Phase 5, EXECUTE: the sanitization maps the raw form values onto the
// workflow's clean input — the web posts the form values, the core
// sanitizes them before the enqueue. A nil values map is the zero payload
// (a dispatch without a form).

func TestSanitizeNilValuesIsZeroPayload(t *testing.T) {
	spec := graph.Spec{Input: graph.Param{Name: "msg", Type: "string"}}
	payload, err := sanitize("hello-world", spec, nil)
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != nil {
		t.Fatalf("payload: got %v, want nil (no form)", payload)
	}
}

func TestSanitizeStringInput(t *testing.T) {
	spec := graph.Spec{Input: graph.Param{Name: "msg", Type: "string"}}
	payload, err := sanitize("hello-world", spec, map[string]string{"msg": "FlowDeck"})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != "FlowDeck" {
		t.Fatalf("payload: got %v, want \"FlowDeck\"", payload)
	}
}

func TestSanitizeIntInput(t *testing.T) {
	spec := graph.Spec{Input: graph.Param{Name: "n", Type: "int"}}
	// the missing value falls back to the input's zero value
	payload, err := sanitize("count", spec, map[string]string{})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != 0 {
		t.Fatalf("payload: got %v, want 0 (zero value)", payload)
	}
	// a value converts to the input's type
	payload, err = sanitize("count", spec, map[string]string{"n": "42"})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != 42 {
		t.Fatalf("payload: got %v, want 42", payload)
	}
	// a value that does not convert is rejected
	_, err = sanitize("count", spec, map[string]string{"n": "x"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}

// A form field that is not in the spec is rejected.
func TestSanitizeUnknownField(t *testing.T) {
	spec := graph.Spec{Input: graph.Param{Name: "msg", Type: "string"}}
	_, err := sanitize("decide", spec, map[string]string{"msg": "x", "bogus": "y"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}

// A workflow without an input takes only an empty form (the zero payload).
func TestSanitizeNoInput(t *testing.T) {
	spec := graph.Spec{}
	payload, err := sanitize("wf", spec, map[string]string{})
	if err != nil {
		t.Fatalf("sanitize: %v", err)
	}
	if payload != nil {
		t.Fatalf("payload: got %v, want nil", payload)
	}
	_, err = sanitize("wf", spec, map[string]string{"msg": "x"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}

// An input type the sanitization does not convert is rejected (the spec
// only understands string and int).
func TestSanitizeUnsupportedType(t *testing.T) {
	spec := graph.Spec{Input: graph.Param{Name: "n", Type: "time.Time"}}
	_, err := sanitize("wf", spec, map[string]string{"n": "1"})
	if !errors.Is(err, ErrInvalidInput) {
		t.Fatalf("sanitize: got %v, want ErrInvalidInput", err)
	}
}
