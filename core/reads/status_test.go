package reads_test

import (
	"errors"
	"testing"

	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
)

// A run the core does not know errors with ErrUnknownExecution (the web
// maps it to a 404).
func TestStatusOfUnknownExecution(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	if _, err := reads.Status("00000000-0000-0000-0000-000000000000"); !errors.Is(err, reads.ErrUnknownExecution) {
		t.Fatalf("Status unknown: got %v, want ErrUnknownExecution", err)
	}
}
