package graph

import (
	"os"
	"testing"
)

// Of returns the stored block model of a built workflow, and nil for a
// name the build never produced (the empty state of the lookup).
func TestOfReturnsTheBuiltGraphAndNilForUnknown(t *testing.T) {
	g, err := New(os.DirFS("../../workflows"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := g.Of("pipeline"); got == nil {
		t.Error("Of(pipeline): got nil, want the built graph")
	}
	if got := g.Of("unknown"); got != nil {
		t.Errorf("Of(unknown): got %+v, want nil", got)
	}
}
