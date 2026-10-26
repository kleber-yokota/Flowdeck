//go:build e2e

package core_test

import (
	"encoding/json"
	"fmt"
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
	"flowdeck/core/status"
)

// E2E against a real Postgres (the e2e job points DBOS_APP_NAME and
// DBOS_SYSTEM_DATABASE_URL at the Postgres 18 service): the same
// env-driven lifecycle as the unit suite, on the production database
// engine. Packages run with -p 1 so only one executor polls the queue at a
// time (a job claimed by a process that has not registered it would fail).
func TestE2EEnqueueRunsToSuccess(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("hello-world", map[string]string{"msg": "FlowDeck"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if st, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v (state %s)", err, st)
	}
}

// A failing run on Postgres: the retries run out, and Detail shows the error
// and the failed step (phase 3, DETAIL).
func TestE2EDetailOfFailedRun(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	st, err := pollState(t, ex.ID, status.Error)
	if err != nil {
		t.Fatalf("poll to error: %v (state %s)", err, st)
	}
	if st != status.Error {
		t.Fatalf("failing run state: got %s, want error", st)
	}
	d, err := core.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if d.State != status.Error || d.FailedStep != "boom-step" {
		t.Fatalf("Detail: %+v", d)
	}
}

// Phase 5, RESUME, E2E: the resume re-enqueues a failed (canceled) run by
// id on Postgres — the same stored input, restarting from the last
// completed step.
func TestE2EResumeRestartsWithSameInput(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("pause", map[string]string{"msg": "same-input"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := pollState(t, ex.ID, status.Running); err != nil {
		t.Fatalf("poll to running: %v", err)
	}
	if err := dbos.CancelWorkflow(core.Ctx(), ex.ID); err != nil {
		t.Fatalf("CancelWorkflow: %v", err)
	}
	if st, err := pollState(t, ex.ID, status.Error); err != nil {
		t.Fatalf("poll to failed: %v (state %s)", err, st)
	}

	ack, err := core.Resume(ex.ID)
	if err != nil {
		t.Fatalf("Resume: %v", err)
	}
	if ack.ID != ex.ID || (ack.State != status.Queued && ack.State != status.Running) {
		t.Fatalf("ACK: %+v", ack)
	}
	if _, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}

	// The same stored input: the restarted run's output echoes the
	// dispatch value.
	var out string
	if err := readStoredOutput(t, ex.ID, &out); err != nil {
		t.Fatalf("stored output: %v", err)
	}
	if out != "paused: same-input" {
		t.Fatalf("output: got %q, want %q", out, "paused: same-input")
	}
}

// readStoredOutput reads the stored output of a finished run (the JSON of
// the workflow's return value).
func readStoredOutput(t *testing.T, id string, out *string) error {
	t.Helper()
	statuses, err := dbos.ListWorkflows(core.Ctx(),
		dbos.WithFilterWorkflowIDs(id),
		dbos.WithFilterLoadInput(false),
		dbos.WithFilterLoadOutput(true),
	)
	if err != nil {
		return fmt.Errorf("ListWorkflows: %w", err)
	}
	if len(statuses) != 1 {
		return fmt.Errorf("ListWorkflows: %d rows, want 1", len(statuses))
	}
	return json.Unmarshal([]byte(statuses[0].Output.(string)), out)
}

// The executed path of a branched run on Postgres (phase 4, STEP 3).
func TestE2EStepsOfBranchedRun(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("decide", map[string]string{"msg": "left"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	if got := mustStepNames(t, ex); len(got) != 2 || got[0] != "base" || got[1] != "left" {
		t.Fatalf("decide left steps: got %v", got)
	}
}
