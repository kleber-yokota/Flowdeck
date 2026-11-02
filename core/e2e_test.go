//go:build e2e

package core_test

import (
	"errors"
	"testing"

	"flowdeck/core"
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
	ex, err := core.Enqueue("hello-world", "FlowDeck")
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
	ex, err := core.Enqueue("failing-test", nil)
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
// Postgres is not resumable today (the SDK's ResumeWorkflow is a no-op on
// ERROR rows — bug.md); the Postgres twin of the unit test above.
func TestE2EResumeOfFailedRun(t *testing.T) {
	testcore.NeedCore(t)
	t.Cleanup(func() { testcore.FlakyFail = false })
	testcore.FlakyFail = true
	ex, err := core.Enqueue("resume-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to failed: %v", err)
	}
	if _, err := core.Resume(ex.ID); !errors.Is(err, core.ErrNotResumable) {
		t.Fatalf("Resume of failed run: got %v, want ErrNotResumable", err)
	}
}

// The executed path of a branched run on Postgres (phase 4, STEP 3).
func TestE2EStepsOfBranchedRun(t *testing.T) {
	testcore.NeedCore(t)
	ex, err := core.Enqueue("decide", "left")
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
