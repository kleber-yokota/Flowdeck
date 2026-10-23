package core

import (
	"context"
	"strings"
	"testing"
)

// The step body is unit-testable without DBOS: only registration needs the context.
func TestGreetStepGreets(t *testing.T) {
	step := GreetStep("FlowDeck")
	result, err := step(context.Background())
	if err != nil {
		t.Fatalf("step: %v", err)
	}
	if !strings.HasPrefix(result, "hello, FlowDeck (") {
		t.Fatalf("result: got %q", result)
	}
}
