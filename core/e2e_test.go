//go:build e2e

package core_test

import (
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/internal/testcore"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testworkflows"
)

// e2eCore returns the env-booted context (this package's TestMain booted
// it from the env vars, once per binary), failing — not skipping — when
// the env is absent: the e2e job provides it, so an absent env is a broken
// run, not a mode.
func e2eCore(t *testing.T) dbos.Context {
	t.Helper()
	if boot.Ctx() == nil {
		t.Fatal("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — the e2e job provides it")
	}
	return boot.Ctx()
}

// E2E against a real database (the e2e job points DBOS_APP_NAME and
// DBOS_SYSTEM_DATABASE_URL at a SQLite file): the same env-driven
// lifecycle as production, on the file database. Packages run with
// -p 1 so only one executor polls the queue at a time (a job claimed by a
// process that has not registered it would fail).
func TestE2EEnqueueRunsToSuccess(t *testing.T) {
	e2eCore(t)
	ex, err := actions.Enqueue("live-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if st, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v (state %s)", err, st)
	}
}

// A failing run on the e2e database: the retries run out, and Detail shows
// the error and the failed step.
func TestE2EDetailOfFailedRun(t *testing.T) {
	e2eCore(t)
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

// The resume of a failed (retries-exhausted) run: the rewind re-enqueues
// the same id and the run reaches success (the e2e twin of the unit test
// above).
func TestE2EResumeOfFailedRun(t *testing.T) {
	e2eCore(t)
	t.Cleanup(func() { testworkflows.FlakyFail = false })
	testworkflows.FlakyFail = true
	ex, err := actions.Enqueue("resume-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to failed: %v", err)
	}
	testworkflows.FlakyFail = false
	if _, err := actions.Resume(ex.ID); err != nil {
		t.Fatalf("Resume of failed run: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
}

// The resume of a cancelled run (the e2e twin of the unit test above): the
// SDK resume re-enqueues the CANCELLED row and the executor runs it to
// success (the noinput-test reaches success fast on the main queue).
func TestE2EResumeOfCancelledRun(t *testing.T) {
	e2eCore(t)
	h, err := boot.Ctx().Enqueue(boot.Ctx(), "frozen", "noinput-test", nil)
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

// The executed path of a branched run.
func TestE2EStepsOfBranchedRun(t *testing.T) {
	e2eCore(t)
	ex, err := actions.Enqueue("fork-test", "left")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	if got := testcore.MustStepNames(t, ex); len(got) != 2 || got[0] != "base" || got[1] != "left" {
		t.Fatalf("fork-test left steps: got %v", got)
	}
}

// The Port's Living field on the e2e database (the e2e twin of
// TestPortLivingRowsTheRunningRun in living_test.go): the fired block
// run is the one row the Port rows.
func TestE2EPortLivingRowsTheRunningRun(t *testing.T) {
	e2eCore(t)
	fired, err := actions.Enqueue("live-test", "block")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, fired.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}

	runs, err := core.NewPort().Living()
	if err != nil {
		t.Fatalf("Living: %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("Living rows: got %d (%+v), want the one running run", len(runs), runs)
	}
	row := runs[0]
	if row.ID != fired.ID || row.Name != "live-test" || row.State != string(status.Running) {
		t.Fatalf("the fired run's row: got %+v, want id %s · name live-test · state running", row, fired.ID)
	}
	if _, err := time.Parse(time.RFC3339, row.CreatedAt); err != nil {
		t.Fatalf("created_at: %q is not RFC 3339: %v", row.CreatedAt, err)
	}

	// Settle (fact F2): the block fire is cancelled inside the test, then
	// an ok fire runs to success — the suite leaves no live-test residue.
	if _, err := actions.Cancel(fired.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	ok, err := actions.Enqueue("live-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue (ok): %v", err)
	}
	if _, err := testcore.PollState(t, ok.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
}

// The children of a child-test run (the e2e twin of
// TestDetailChildrenOfChildDispatchingRun): the detail lists the child
// inner-test with its id and badge state.
func TestE2EDetailChildrenOfChildTestRun(t *testing.T) {
	e2eCore(t)
	ex, err := actions.Enqueue("child-test", "FlowDeck")
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
	if c.ID == "" || c.Name != "inner-test" || c.State != status.Success {
		t.Fatalf("child: got %+v, want inner-test (success)", c)
	}
}
