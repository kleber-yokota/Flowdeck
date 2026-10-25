package core

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
	_ "modernc.org/sqlite"

	"flowdeck/core/status"
)

// failingTestWorkflow is a workflow that fails in a named step — it
// exercises the failure path of Detail (error + failed step).
// failingTestStep is a step that always fails (the Detail failure path).
func failingTestStep() dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return "", errors.New("boom: step failed")
	}
}

func failingTestWorkflow(ctx dbos.Context, _ string) (string, error) {
	_, err := dbos.RunAsStep(ctx, failingTestStep(), dbos.WithStepName("boom-step"))
	if err != nil {
		return "", err
	}
	return "", nil
}

// Executions lists the runs of a workflow by name (phase 3, LIST).
func TestExecutionsListsRunsOfWorkflow(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	handle, err := dbos.RunWorkflow(theCore.dbosCtx, HelloWorld, "one")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if _, err := handle.GetResult(); err != nil {
		t.Fatalf("GetResult: %v", err)
	}

	exs, err := Executions("hello-world")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(exs) != 1 {
		t.Fatalf("Executions: got %d runs, want 1: %v", len(exs), exs)
	}
	if exs[0].ID != handle.GetWorkflowID() {
		t.Fatalf("Executions: got %s, want %s", exs[0].ID, handle.GetWorkflowID())
	}
	if exs[0].State != status.Success {
		t.Fatalf("Executions: got state %s, want %s", exs[0].State, status.Success)
	}
}

// A name with no runs is an empty list, not an error.
func TestExecutionsUnknownNameIsEmpty(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	exs, err := Executions("no-such-workflow")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(exs) != 0 {
		t.Fatalf("Executions: got %v, want no runs", exs)
	}
}

// Executions before Launch must fail cleanly (core not up yet).
func TestExecutionsWithoutLaunchErrors(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if _, err := Executions("hello-world"); !errors.Is(err, ErrNotLaunched) {
		t.Fatalf("Executions before Launch: got %v, want ErrNotLaunched", err)
	}
}

// Detail reads a successful run (phase 3, DETAIL): the state, the first
// execution date (created_at), no error.
// Executions lists the runs newest first, each run carrying its first
// execution date (created_at).
func TestExecutionsNewestFirstWithCreatedAt(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	first, err := dbos.RunWorkflow(theCore.dbosCtx, HelloWorld, "one")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if _, err := first.GetResult(); err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	second, err := dbos.RunWorkflow(theCore.dbosCtx, HelloWorld, "two")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if _, err := second.GetResult(); err != nil {
		t.Fatalf("GetResult: %v", err)
	}

	exs, err := Executions("hello-world")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(exs) != 2 {
		t.Fatalf("Executions: got %d runs, want 2: %v", len(exs), exs)
	}
	for i, ex := range exs {
		if ex.CreatedAt.IsZero() {
			t.Fatalf("Executions[%d]: zero CreatedAt: %+v", i, ex)
		}
	}
	// Newest first (created_at descending)
	if exs[0].CreatedAt.Before(exs[1].CreatedAt) {
		t.Fatalf("Executions not newest first: %v before %v", exs[0].CreatedAt, exs[1].CreatedAt)
	}
	// The newest run is the second one dispatched
	if exs[0].ID != second.GetWorkflowID() {
		t.Fatalf("Executions[0]: got %s, want %s", exs[0].ID, second.GetWorkflowID())
	}
}

func TestDetailSuccess(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	handle, err := dbos.RunWorkflow(theCore.dbosCtx, HelloWorld, "one")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if _, err := handle.GetResult(); err != nil {
		t.Fatalf("GetResult: %v", err)
	}

	d, err := Detail(handle.GetWorkflowID())
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if d.State != status.Success {
		t.Fatalf("Detail: got state %s, want %s", d.State, status.Success)
	}
	if d.CreatedAt.IsZero() {
		t.Fatal("Detail: CreatedAt is zero, want the run's created_at")
	}
	if d.Error != "" {
		t.Fatalf("Detail: unexpected error: %q", d.Error)
	}
	if d.FailedStep != "" {
		t.Fatalf("Detail: unexpected failed step: %q", d.FailedStep)
	}
}

// Detail reads a failed run (phase 3: "failure → error + failed step"): the
// error message and the step that failed.
func TestDetailFailureReportsErrorAndFailedStep(t *testing.T) {
	bootSQLiteCoreForTest(t)
	registerTestWorkflow("failing-test", failingTestWorkflow)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	handle, err := dbos.RunWorkflow(theCore.dbosCtx, failingTestWorkflow, "x")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if _, err := handle.GetResult(); err == nil {
		t.Fatal("expected the workflow to fail")
	}

	d, err := Detail(handle.GetWorkflowID())
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if d.State != status.Error {
		t.Fatalf("Detail: got state %s, want %s", d.State, status.Error)
	}
	if !strings.Contains(d.Error, "boom: step failed") {
		t.Fatalf("Detail: got error %q, want the step error", d.Error)
	}
	if d.FailedStep != "boom-step" {
		t.Fatalf("Detail: got failed step %q, want %q", d.FailedStep, "boom-step")
	}
	if d.Attempts < 1 {
		t.Fatalf("Detail: got %d attempts, want >= 1", d.Attempts)
	}
}

// Steps returns the executed path of a branched run: the steps in
// execution order (step_id asc) — the branch of the if the run took
// (phase 4, STEP 3). The not-taken branch is not in the path.
func TestStepsReturnsTakenBranch(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	tests := []struct {
		payload string
		want    []string
	}{
		{"", []string{"base", "right"}},
		{"left", []string{"base", "left"}},
	}
	for _, tt := range tests {
		handle, err := dbos.RunWorkflow(theCore.dbosCtx, Decide, tt.payload)
		if err != nil {
			t.Fatalf("RunWorkflow: %v", err)
		}
		if _, err := handle.GetResult(); err != nil {
			t.Fatalf("GetResult: %v", err)
		}
		steps, err := Steps(handle.GetWorkflowID())
		if err != nil {
			t.Fatalf("Steps: %v", err)
		}
		if len(steps) != len(tt.want) {
			t.Fatalf("payload %q: got %v, want %v", tt.payload, steps, tt.want)
		}
		for i, s := range steps {
			if s.Name != tt.want[i] {
				t.Fatalf("payload %q: Steps[%d] got %q, want %q", tt.payload, i, s.Name, tt.want[i])
			}
			if s.State != status.Success {
				t.Fatalf("Steps[%d]: got state %s, want %s", i, s.State, status.Success)
			}
		}
	}
}

// The executed path of a failed run is the single failed step, with the
// failed state (phase 4, STEP 3).
func TestStepsOfFailedRun(t *testing.T) {
	bootSQLiteCoreForTest(t)
	registerTestWorkflow("failing-test", failingTestWorkflow)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	handle, err := dbos.RunWorkflow(theCore.dbosCtx, failingTestWorkflow, "x")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if _, err := handle.GetResult(); err == nil {
		t.Fatal("expected the workflow to fail")
	}

	steps, err := Steps(handle.GetWorkflowID())
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	if len(steps) != 1 {
		t.Fatalf("Steps: got %v, want one step", steps)
	}
	if steps[0].Name != "boom-step" {
		t.Fatalf("Steps[0]: got %q, want %q", steps[0].Name, "boom-step")
	}
	if steps[0].State != status.Error {
		t.Fatalf("Steps[0]: got state %s, want %s", steps[0].State, status.Error)
	}
}

// Steps on an ID the core does not know returns ErrUnknownExecution (the
// web maps it to 404).
func TestStepsUnknownExecution(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	if _, err := Steps("00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrUnknownExecution) {
		t.Fatalf("Steps: got %v, want ErrUnknownExecution", err)
	}
}

// Steps before Launch must fail cleanly (core not up yet).
func TestStepsWithoutLaunchErrors(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if _, err := Steps("abc"); !errors.Is(err, ErrNotLaunched) {
		t.Fatalf("Steps before Launch: got %v, want ErrNotLaunched", err)
	}
}

// Detail on an ID the core does not know returns ErrUnknownExecution (the
// web maps it to 404).
func TestDetailUnknownExecution(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	if _, err := Detail("00000000-0000-0000-0000-000000000000"); !errors.Is(err, ErrUnknownExecution) {
		t.Fatalf("Detail: got %v, want ErrUnknownExecution", err)
	}
}

// A run that existed but was deleted from the database ("nao existe mais"):
// Detail must return ErrUnknownExecution (the web maps it to 404) and the
// run must no longer appear in Executions.
func TestDetailDeletedExecution(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "core-test.db")
	bootCoreForTest(t, "sqlite:"+dbPath)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	handle, err := dbos.RunWorkflow(theCore.dbosCtx, HelloWorld, "one")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	if _, err := handle.GetResult(); err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	id := handle.GetWorkflowID()

	// Delete the run from the database: children first, then the status row.
	// The live queue runner may hold a short write lock — busy_timeout makes
	// the delete wait for it instead of failing with SQLITE_BUSY.
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer db.Close()
	for _, table := range []string{"operation_outputs", "workflow_status"} {
		if _, err := db.Exec("DELETE FROM "+table+" WHERE workflow_uuid = ?", id); err != nil {
			t.Fatalf("DELETE FROM %s: %v", table, err)
		}
	}

	if _, err := Detail(id); !errors.Is(err, ErrUnknownExecution) {
		t.Fatalf("Detail: got %v, want ErrUnknownExecution", err)
	}

	exs, err := Executions("hello-world")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(exs) != 0 {
		t.Fatalf("Executions: got %v, want no runs", exs)
	}
}

// Detail before Launch must fail cleanly (core not up yet).
func TestDetailWithoutLaunchErrors(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if _, err := Detail("abc"); !errors.Is(err, ErrNotLaunched) {
		t.Fatalf("Detail before Launch: got %v, want ErrNotLaunched", err)
	}
}
