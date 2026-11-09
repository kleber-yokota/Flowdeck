package queue

import (
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
)

// TestQueueRunsOneAtATime: the queue workflow runs one at a time (the
// serial queue, concurrency 1) — the second dispatch stays queued while
// the first run is running.
func TestQueueRunsOneAtATime(t *testing.T) {
	testcore.BootTest(t, "..", Register)
	ex1, err := actions.Enqueue("queue", nil)
	if err != nil {
		t.Fatalf("enqueue 1: %v", err)
	}
	ex2, err := actions.Enqueue("queue", nil)
	if err != nil {
		t.Fatalf("enqueue 2: %v", err)
	}
	if _, err := testcore.PollState(t, ex1.ID, status.Running); err != nil {
		t.Fatalf("poll 1 to running: %v", err)
	}
	// The second run is not claimed while the first is running — it stays
	// queued (the serialization).
	st, err := reads.Status(ex2.ID)
	if err != nil {
		t.Fatalf("status 2: %v", err)
	}
	if st != status.Queued {
		t.Fatalf("state 2: got %s, want queued", st)
	}
	// The first run ends successful (the 5s sleep step)...
	if _, err := testcore.PollState(t, ex1.ID, status.Success); err != nil {
		t.Fatalf("poll 1 to success: %v", err)
	}
	// ...and the test owns the live test: the second run is cancelled.
	if _, err := actions.Cancel(ex2.ID); err != nil {
		t.Fatalf("cancel 2: %v", err)
	}
}

// TestQueueResumeStaysSerial: the resume of a cancelled queue run
// re-enqueues it on the serial queue (the same queue selection as the
// dispatch).
func TestQueueResumeStaysSerial(t *testing.T) {
	c := testcore.BootTest(t, "..", Register)
	ex, err := actions.Enqueue("queue", nil)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if _, err := actions.Cancel(ex.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := actions.Resume(ex.ID); err != nil {
		t.Fatalf("resume: %v", err)
	}
	statuses, err := reads.ListByID(c, ex.ID)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if statuses[0].QueueName != boot.SerialQueueName {
		t.Fatalf("queue: got %q, want %q", statuses[0].QueueName, boot.SerialQueueName)
	}
	if _, err := actions.Cancel(ex.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
}
