package core_test

import (
	"errors"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
)

// The 14th Port field (the failed read): the Port wires Failed to the live
// read — the wiring reaches the guard of reads.Failed (not up →
// boot.ErrNotLaunched: the same guard the siblings' fields carry).
func TestPortWiresFailed(t *testing.T) {
	p := core.NewPort()
	if p.Failed == nil {
		t.Fatal("p.Failed is nil — the field is not wired")
	}
	if _, err := p.Failed(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("err: %v, want boot.ErrNotLaunched", err)
	}
}
