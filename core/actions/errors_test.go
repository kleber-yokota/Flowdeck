package actions_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
	"flowdeck/core/testworkflows"
)

// The actions act on runs of a booted core: a not-launched core is a
// clean ErrNotLaunched (the web maps it to a 503).
func TestActionsOfUnbootedCore(t *testing.T) {
	if _, err := actions.Enqueue("live-test", "FlowDeck"); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Enqueue unbooted: got %v, want ErrNotLaunched", err)
	}
	if _, err := actions.Cancel("00000000-0000-0000-0000-000000000000"); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Cancel unbooted: got %v, want ErrNotLaunched", err)
	}
	if _, err := actions.Resume("00000000-0000-0000-0000-000000000000"); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Resume unbooted: got %v, want ErrNotLaunched", err)
	}
}

// serialWorkflow is the serial dispatch: the workflow named by the
// dispatch table ("queue") — its dispatches go through the serial queue
// (concurrency 1), not the main queue.
func serialWorkflow(ctx dbos.Context, msg string) (string, error) {
	return msg, nil
}

// The serial dispatch: the dispatch table routes the run to the serial
// queue — the run goes onto the serial queue, one at a time.
func TestEnqueueOfASerialWorkflow(t *testing.T) {
	testcore.BootTestDB(t, "../testworkflows", func(c dbos.Context) {
		testworkflows.RegisterTestWorkflows(c)
		dbos.RegisterWorkflow(c, serialWorkflow, dbos.WithWorkflowName("queue"))
	})
	ex, err := actions.Enqueue("queue", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue serial: %v", err)
	}
	if ex.State != status.Queued {
		t.Fatalf("state: got %s, want queued", ex.State)
	}
}

// The dispatch writes the new run's status row: with the status table
// dropped (second connection), the enqueue fails — a real DB read/write
// failure, not an unknown workflow.
func TestEnqueueReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	testcore.DropTable(t, dbfile, "workflow_status")
	if _, err := actions.Enqueue("live-test", "FlowDeck"); err == nil {
		t.Fatal("Enqueue after dropped status table: got nil, want a DB failure")
	}
}

// failedRun enqueues the failing fixture and waits until it fails: the
// resume's read-failure states need a run that is already failed.
func failedRun(t *testing.T, dbfile string) string {
	t.Helper()
	ex, err := actions.Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue failing: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to error: %v", err)
	}
	return ex.ID
}

// The resume reads the run's status row first: with the status table
// dropped, the resume fails with a DB error (not an unknown execution).
func TestResumeReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	id := failedRun(t, dbfile)
	testcore.DropTable(t, dbfile, "workflow_status")
	if _, err := actions.Resume(id); err == nil {
		t.Fatal("Resume after dropped status table: got nil, want a read failure")
	}
}

// A failed run in a state the SDK cannot produce is rejected by the
// resume's read: the guard maps the status and the unmapped state is an
// error, not a guess.
func TestResumeOfAnUnmappedRow(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	id := failedRun(t, dbfile)
	testcore.SetUnmappedState(t, dbfile, id)
	if _, err := actions.Resume(id); err == nil || !strings.Contains(err.Error(), "unmapped") {
		t.Fatalf("Resume unmapped row: got %v, want the unmapped-state rejection", err)
	}
}

// The rewind reads the run's step rows to find the cut: with the step
// rows dropped (the second connection), the rewind's read fails — the
// resume fails with a DB error before it rewinds.
func TestResumeRewindStepsReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	id := failedRun(t, dbfile)
	testcore.DropTable(t, dbfile, "operation_outputs")
	if _, err := actions.Resume(id); err == nil {
		t.Fatal("Resume after dropped step rows: got nil, want a read failure")
	}
}

// The rewind also unpublishes the run's events: with the events-history
// table dropped, the rewind itself fails — the resume surfaces the DB
// error (the run is not re-enqueued on a broken DB).
func TestResumeRewindFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	id := failedRun(t, dbfile)
	testcore.DropTable(t, dbfile, "workflow_events_history")
	if _, err := actions.Resume(id); err == nil {
		t.Fatal("Resume after dropped events-history: got nil, want a rewind failure")
	}
}

// The SDK cancel retries its begin-immediate transaction with a backoff:
// with a second connection holding the write lock (BEGIN IMMEDIATE), the
// first attempt fails busy; a row deleted during the backoff makes the
// retry's update match nothing, and the cancel returns the wrapped
// not-exist error — the guard read had succeeded (the failure is inside
// the SDK cancel). The run is enqueued on a queue no runner listens to,
// so the executor never dequeues it: the seam is the cancel's busy-retry
// itself, not a race with the queue runner for the write lock.
func TestCancelOfARunDeletedDuringTheRetryBackoff(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	// The busy_timeout DSN (OpenSecondConn) lets the BEGIN IMMEDIATE wait
	// out the queue runner's in-flight dequeue writes.
	db := testcore.OpenSecondConn(t, dbfile)

	for attempt := 0; attempt < 10; attempt++ {
		// Enqueue directly on a queue with no runner: the run row is never
		// dequeued, so the second conn's BEGIN IMMEDIATE always wins the
		// write lock — no race with the queue runner.
		c, err := boot.CtxOrErr()
		if err != nil {
			t.Fatalf("ctx: %v", err)
		}
		handle, err := c.Enqueue(c, "no-runner", "block-test", 1)
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		id := handle.GetWorkflowID()
		conn, err := db.Conn(context.Background())
		if err != nil {
			t.Fatalf("second conn: %v", err)
		}
		if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
			t.Fatalf("begin immediate: %v", err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			time.Sleep(20 * time.Millisecond)
			if _, err := conn.ExecContext(context.Background(), "DELETE FROM workflow_status WHERE workflow_uuid = ?", id); err != nil {
				t.Errorf("delete: %v", err)
			}
			if _, err := conn.ExecContext(context.Background(), "COMMIT"); err != nil {
				t.Errorf("commit: %v", err)
			}
		}()
		if _, err := actions.Cancel(id); err != nil && strings.Contains(err.Error(), "does not exist") {
			<-done
			return
		}
		<-done
	}
	t.Fatal("the delete-in-the-backoff seam was never hit in 10 attempts")
}

// The SDK resume re-enqueues the cancelled row in a begin-immediate
// transaction with the same retry backoff: with a second connection
// holding the write lock, the first attempt fails busy; a row deleted
// during the backoff makes the retry match nothing, and the resume
// returns the wrapped not-exist error — the guard read had succeeded
// (the failure is inside the SDK resume).
func TestResumeOfARunDeletedDuringTheRetryBackoff(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	// The busy_timeout DSN (OpenSecondConn) lets the BEGIN IMMEDIATE wait
	// out the queue runner's in-flight dequeue writes.
	db := testcore.OpenSecondConn(t, dbfile)
	for attempt := 0; attempt < 10; attempt++ {
		// Enqueue directly on a queue with no runner: the run row is never
		// dequeued, so the second conn's BEGIN IMMEDIATE always wins the
		// write lock — no race with the queue runner.
		c, err := boot.CtxOrErr()
		if err != nil {
			t.Fatalf("ctx: %v", err)
		}
		handle, err := c.Enqueue(c, "no-runner", "block-test", 1)
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		id := handle.GetWorkflowID()
		if _, err := actions.Cancel(id); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
		conn, err := db.Conn(context.Background())
		if err != nil {
			t.Fatalf("second conn: %v", err)
		}
		if _, err := conn.ExecContext(context.Background(), "BEGIN IMMEDIATE"); err != nil {
			t.Fatalf("begin immediate: %v", err)
		}
		done := make(chan struct{})
		go func() {
			defer close(done)
			time.Sleep(20 * time.Millisecond)
			if _, err := conn.ExecContext(context.Background(), "DELETE FROM workflow_status WHERE workflow_uuid = ?", id); err != nil {
				t.Errorf("delete: %v", err)
			}
			if _, err := conn.ExecContext(context.Background(), "COMMIT"); err != nil {
				t.Errorf("commit: %v", err)
			}
		}()
		if _, err := actions.Resume(id); err != nil && strings.Contains(err.Error(), "does not exist") {
			<-done
			return
		}
		<-done
	}
	t.Fatal("the delete-in-the-backoff seam was never hit in 10 attempts")
}
