package core_test

import (
	"errors"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
)

// The failed read (the Port embeds it — a promoted method): the wiring
// reaches the reads.Failed guard (not up → boot.ErrNotLaunched).
func TestPortWiresFailed(t *testing.T) {
	p := core.NewPort()
	if _, err := p.Failed(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("err: %v, want boot.ErrNotLaunched", err)
	}
}
