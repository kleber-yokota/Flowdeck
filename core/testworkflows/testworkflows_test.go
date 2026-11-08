package testworkflows_test

import (
	"testing"

	"flowdeck/core/testworkflows"
)

// The Workflows list is the registration itself: one entry per test
// workflow (ten) — the test names what is registered, and only that
// boots.
func TestWorkflowsList(t *testing.T) {
	if len(testworkflows.Workflows) != 10 {
		t.Fatalf("Workflows list: got %d entries, want 10", len(testworkflows.Workflows))
	}
}

// The Queues list is the named queues: frozen and live-frozen, both with
// worker concurrency 0 (the deterministic queued-state fixtures) — one
// registration, named, in the list.
func TestQueuesList(t *testing.T) {
	if len(testworkflows.Queues) != 2 {
		t.Fatalf("Queues list: got %d entries, want 2", len(testworkflows.Queues))
	}
	for i, want := range []string{"frozen", "live-frozen"} {
		q := testworkflows.Queues[i]
		if q.Name != want {
			t.Fatalf("Queues[%d].Name: got %q, want %q", i, q.Name, want)
		}
		if q.Concurrency != 0 {
			t.Fatalf("Queues[%d].Concurrency: got %d, want 0", i, q.Concurrency)
		}
	}
}
