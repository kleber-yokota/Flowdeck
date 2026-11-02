package core_test

import (
	"errors"
	"testing"

	"flowdeck/core"
	"flowdeck/core/internal/testcore"
)

// A run the core does not know errors with ErrUnknownExecution (the web
// maps it to a 404).
func TestStatusOfUnknownExecution(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := core.Status("00000000-0000-0000-0000-000000000000"); !errors.Is(err, core.ErrUnknownExecution) {
		t.Fatalf("Status unknown: got %v, want ErrUnknownExecution", err)
	}
}
