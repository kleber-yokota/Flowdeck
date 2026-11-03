//go:build e2e

package core_test

import (
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
)

// E2E against a real Postgres (the e2e job points DBOS_APP_NAME and
// DBOS_SYSTEM_DATABASE_URL at the Postgres 18 service): the same
// env-driven lifecycle as the unit suite, on the production database
// engine. Packages run with -p 1 so only one executor polls the queue at a
// time (a job claimed by a process that has not registered it would fail).
func TestE2EEnqueueRunsToSuccess(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("hello-world", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if st, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v (state %s)", err, st)
	}
}

// A failing run on Postgres: the retries run out, and Detail shows the error
// and the failed step (phase 3, DETAIL).
func TestE2EDetailOfFailedRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	st, err := testcore.PollState(t, ex.ID, status.Error)
	if err != nil {
		t.Fatalf("poll to error: %v (state %s)", err, st)
	}
	if st != status.Error {
		t.Fatalf("failing run state: got %s, want error", st)
	}
	d, err := reads.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if d.ExecutionDetail.State != status.Error || d.FailedStep != "boom-step" {
		t.Fatalf("Detail: %+v", d)
	}
}

// Phase 5, RESUME, E2E: the resume of a failed (retries-exhausted) run on
// Postgres — the rewind re-enqueues the same id and the run reaches
// success (the Postgres twin of the unit test above).
func TestE2EResumeOfFailedRun(t *testing.T) {
	testcore.NeedCore(t)
	t.Cleanup(func() { testcore.FlakyFail = false })
	testcore.FlakyFail = true
	ex, err := actions.Enqueue("resume-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to failed: %v", err)
	}
	testcore.FlakyFail = false
	if _, err := actions.Resume(ex.ID); err != nil {
		t.Fatalf("Resume of failed run: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
}

// The resume of a cancelled run on Postgres (the Postgres twin of the unit
// test above): the SDK resume re-enqueues the CANCELLED row and the
// executor runs it to success.
func TestE2EResumeOfCancelledRun(t *testing.T) {
	testcore.NeedCore(t)
	if _, err := dbos.RegisterQueue(boot.Ctx(), "frozen", dbos.WithWorkerConcurrency(0)); err != nil {
		t.Fatalf("RegisterQueue: %v", err)
	}
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "pause", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := h.GetWorkflowID()
	if _, err := actions.Cancel(id); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if _, err := actions.Resume(id); err != nil {
		t.Fatalf("Resume of cancelled: %v", err)
	}
	if _, err := testcore.PollState(t, id, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
}

// The executed path of a branched run on Postgres (phase 4, STEP 3).
func TestE2EStepsOfBranchedRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("decide", "left")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	if got := testcore.MustStepNames(t, ex); len(got) != 2 || got[0] != "base" || got[1] != "left" {
		t.Fatalf("decide left steps: got %v", got)
	}
}

// The children of a kitchen run on Postgres (the Postgres twin of
// TestDetailChildrenOfChildDispatchingRun): the detail lists the child
// pipeline with its id and badge state.
func TestE2EDetailChildrenOfKitchenRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := actions.Enqueue("kitchen", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	d, err := reads.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(d.Children) != 1 {
		t.Fatalf("children: got %v, want exactly one", d.Children)
	}
	c := d.Children[0]
	if c.ID == "" || c.Name != "pipeline" || c.State != status.Success {
		t.Fatalf("child: got %+v, want pipeline (success)", c)
	}
}
