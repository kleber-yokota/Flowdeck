package reads_test

import (
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// The live read counts the runs by their current state (the three live
// boxes): every run running right now is in the running box and every run
// in the queue right now is in the queued box (firing the same workflow
// several times counts each run — no deduplication by workflow), and
// every workflow whose latest run is an error or a cancelled one is in
// the failed box; a latest success with no live run is in no box. Every
// test owns the test-only workflow "live-test" (never a workflows/
// workflow), starts from the quiesced baseline s0, asserts all three
// counts at every checkpoint, and ends with live-test's latest run in
// SUCCESS (start state = end state: the live-test contribution is zero).

// quiescedLive waits for the suite to settle — no in-flight run: no
// PENDING row and no ENQUEUED row on the main queue (the executor
// claims them; an ENQUEUED row on a frozen queue — worker concurrency 0 —
// is stable and stays in the baseline) — and returns the live snapshot:
// the baseline s0 of the tests. The shared suite DB holds runs from the
// other packages, and the detail tests assert runs while they are still
// live: their leftover runs settle here.
func quiescedLive(t *testing.T) reads.Live {
	t.Helper()
	for i := 0; ; i++ {
		rows, err := dbos.ListWorkflows(boot.Ctx(),
			dbos.WithFilterStatus(dbos.WorkflowStatusEnqueued, dbos.WorkflowStatusPending),
			dbos.WithFilterLoadInput(false),
			dbos.WithFilterLoadOutput(false),
		)
		if err != nil {
			t.Fatalf("ListWorkflows: %v", err)
		}
		inFlight := 0
		for _, r := range rows {
			if r.Status == dbos.WorkflowStatusPending || r.QueueName == boot.QueueName {
				inFlight++
			}
		}
		if inFlight == 0 {
			live, err := reads.LiveCounts()
			if err != nil {
				t.Fatalf("LiveCounts: %v", err)
			}
			return live
		}
		if i >= 300 {
			t.Fatalf("no settled live read: %d in-flight runs", inFlight)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// liveDelta asserts the live counts at a checkpoint: the baseline s0 plus
// the expected delta of the live-test contribution.
func liveDelta(t *testing.T, s0 reads.Live, dRunning, dQueued, dFailed int) {
	t.Helper()
	got, err := reads.LiveCounts()
	if err != nil {
		t.Fatalf("LiveCounts: %v", err)
	}
	want := reads.Live{Running: s0.Running + dRunning, Queued: s0.Queued + dQueued, Failed: s0.Failed + dFailed}
	if got != want {
		t.Fatalf("live counts: got %+v, want %+v", got, want)
	}
}

// A success run enters no box: the three counts are the baseline.
func TestLiveSuccessRunEntersNoBox(t *testing.T) {
	testcore.NeedCore(t)
	s0 := quiescedLive(t)
	ex, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	liveDelta(t, s0, 0, 0, 0)
}

// An error run is in the failed box until the latest run is a success
// again: a past error with a latest success is the zero contribution.
func TestLiveErrorRunCountedUntilLatestSuccess(t *testing.T) {
	testcore.NeedCore(t)
	s0 := quiescedLive(t)
	ex, err := actions.Enqueue("live-test", "err")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to error: %v", err)
	}
	liveDelta(t, s0, 0, 0, 1)
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	liveDelta(t, s0, 0, 0, 0)
}

// A running run is in the running box; the cancel of the PENDING run is
// the failed box (cancelled); the latest success is the baseline again.
func TestLiveRunningRunCountedUntilCancelled(t *testing.T) {
	testcore.NeedCore(t)
	s0 := quiescedLive(t)
	ex, err := actions.Enqueue("live-test", "block")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	liveDelta(t, s0, 1, 0, 0)
	if _, err := actions.Cancel(ex.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	liveDelta(t, s0, 0, 0, 1)
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	liveDelta(t, s0, 0, 0, 0)
}

// A workflow that is running and enqueued at the same time (a PENDING run
// and an ENQUEUED run) is in both boxes: the boxes count the current
// state of the workflow, not its latest run.
func TestLiveRunningAndQueuedAtOnceCountBothBoxes(t *testing.T) {
	testcore.NeedCore(t)
	s0 := quiescedLive(t)
	block, err := actions.Enqueue("live-test", "block")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, block.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	frozen, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	// The workflow is in the running box and the queued box at the same
	// time (the latest run is the ENQUEUED one — the boxes do not read it
	// as the single truth).
	liveDelta(t, s0, 1, 1, 0)
	if _, err := actions.Cancel(frozen.GetWorkflowID()); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := actions.Cancel(block.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	// The two cancelled runs: one workflow in the failed box (the latest
	// run is a cancelled one, deduplicated by name).
	liveDelta(t, s0, 0, 0, 1)
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	liveDelta(t, s0, 0, 0, 0)
}

// Firing the same workflow several times counts every run running at
// the same time (no deduplication by workflow): three block fires on the
// main queue (no worker limit — the executor claims them all) are three
// running runs (the block steps unblock at the cancels). The three cancels
// are one failed workflow (the failed box keeps the latest run per name)
// and the latest success is the baseline again.
func TestLiveRepeatedEnqueuesCountEveryRunningRun(t *testing.T) {
	testcore.NeedCore(t)
	s0 := quiescedLive(t)
	ids := make([]string, 3)
	for i := range ids {
		ex, err := actions.Enqueue("live-test", "block")
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		ids[i] = ex.ID
	}
	for _, id := range ids {
		if _, err := testcore.PollState(t, id, status.Running); err != nil {
			t.Fatalf("poll to running: %v", err)
		}
	}
	liveDelta(t, s0, 3, 0, 0)
	for _, id := range ids {
		if _, err := actions.Cancel(id); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	}
	liveDelta(t, s0, 0, 0, 1)
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	liveDelta(t, s0, 0, 0, 0)
}

// Firing the same workflow several times counts every run in the queue
// (no deduplication by workflow): three fires on the frozen queue
// (worker concurrency 0 — the executor never claims them) are three
// queued runs.
func TestLiveRepeatedEnqueuesCountEveryQueuedRun(t *testing.T) {
	testcore.NeedCore(t)
	s0 := quiescedLive(t)
	ids := make([]string, 3)
	for i := range ids {
		ex, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
		if err != nil {
			t.Fatalf("Enqueue: %v", err)
		}
		ids[i] = ex.GetWorkflowID()
	}
	liveDelta(t, s0, 0, 3, 0)
	for _, id := range ids {
		if _, err := actions.Cancel(id); err != nil {
			t.Fatalf("Cancel: %v", err)
		}
	}
	liveDelta(t, s0, 0, 0, 1)
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	liveDelta(t, s0, 0, 0, 0)
}

// A queued run (on the frozen queue: worker concurrency 0, the executor
// never claims it) is in the queued box; the cancel is the failed box;
// the latest success on the main queue is the baseline again.
func TestLiveQueuedRunCountedUntilCancelled(t *testing.T) {
	testcore.NeedCore(t)
	s0 := quiescedLive(t)
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "live-frozen", "live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := h.GetWorkflowID()
	st, err := reads.Status(id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if st != status.Queued {
		t.Fatalf("state: got %s, want queued", st)
	}
	liveDelta(t, s0, 0, 1, 0)
	if _, err := actions.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	liveDelta(t, s0, 0, 0, 1)
	ok, err := actions.Enqueue("live-test", "ok")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	liveDelta(t, s0, 0, 0, 0)
}
