package registry_test

import (
	"testing"

	"flowdeck/core/registry"
)

// The unbooted registry: without a booted core the List is nil — the web's
// "No workflows registered" empty state — and the Spec of any name is the
// unknown-workflow error.

// The List is nil before any boot (the web renders its empty state).
func TestListUnbooted(t *testing.T) {
	if got := registry.List(); got != nil {
		t.Errorf("List(unbooted): got %v, want nil", got)
	}
}
