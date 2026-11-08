package registry_test

import (
	"errors"
	"os"
	"testing"

	"flowdeck/core/boot"
	"flowdeck/core/registry"
)

// The core exposes the spec of a registered workflow — the input and the
// steps' default values, parsed from the source at boot (the web renders
// them on top of the workflow page). The Spec reads the paths graph, so
// the test builds it from the testworkflows source (no boot needed).
func TestSpecOfRegisteredWorkflow(t *testing.T) {
	if err := boot.SetWorkflowsSource(os.DirFS("../testworkflows")); err != nil {
		t.Fatalf("set workflows source: %v", err)
	}
	spec, err := registry.Spec("fork-test")
	if err != nil {
		t.Fatalf("registry.Spec(fork-test): %v", err)
	}
	if len(spec.Inputs) != 1 {
		t.Fatalf("fork-test inputs: got %v, want 1", spec.Inputs)
	}
	if in := spec.Inputs[0]; in.Name != "msg" || in.Type != "string" {
		t.Fatalf("fork-test input: got %+v, want {msg string}", in)
	}
	wantSteps := map[string]string{"base": "base", "left": "left", "right": "right"}
	if len(spec.Steps) != len(wantSteps) {
		t.Fatalf("fork-test steps: got %v, want %v", spec.Steps, wantSteps)
	}
	for _, s := range spec.Steps {
		if wantSteps[s.Name] != s.Default {
			t.Fatalf("fork-test step %s: got default %q, want %q", s.Name, s.Default, wantSteps[s.Name])
		}
	}
}

// Spec of a name that is not in the source errors with ErrUnknownWorkflow
// (the web maps it to a 404).
func TestSpecOfUnknownWorkflow(t *testing.T) {
	if _, err := registry.Spec("no-such-workflow"); !errors.Is(err, registry.ErrUnknownWorkflow) {
		t.Fatalf("Spec unknown: got %v, want ErrUnknownWorkflow", err)
	}
}
