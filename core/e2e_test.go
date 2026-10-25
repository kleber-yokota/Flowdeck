//go:build e2e

package core

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/status"
)

// Integration test: runs the example workflow (with step) against a real
// Postgres, using the core singleton. Run with:
//
//	DBOS_E2E_DATABASE_URL=postgres://user:pass@host:5432/db \
//	  go test -tags e2e -run TestE2E ./...
func TestE2EHelloWorldRunsStep(t *testing.T) {
	dbURL := os.Getenv("DBOS_E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("DBOS_E2E_DATABASE_URL not set — skipping e2e test")
	}

	bootCoreForTest(t, dbURL)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	names := map[string]bool{}
	for _, w := range List() {
		names[w.Name] = true
	}
	if !names["hello-world"] {
		t.Fatalf("registry missing hello-world: %v", names)
	}

	handle, err := dbos.RunWorkflow(theCore.dbosCtx, HelloWorld, "FlowDeck")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if !strings.HasPrefix(result, "hello, FlowDeck") {
		t.Fatalf("step result: got %q", result)
	}
}

// A dispatched run that fails ends in the error state, with the error and
// the failed step readable by Detail (phase 3, DETAIL) and the run listed
// by Executions (phase 3, LIST).
func TestE2EDetailOfFailedRun(t *testing.T) {
	dbURL := os.Getenv("DBOS_E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("DBOS_E2E_DATABASE_URL not set — skipping e2e test")
	}

	bootCoreForTest(t, dbURL)
	registerTestWorkflow("failing-test", failingTestWorkflow)
	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	ex, err := Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if ex.CreatedAt.IsZero() {
		t.Fatal("Enqueue returned a zero CreatedAt")
	}
	id := ex.ID

	// The executor runs the run to its terminal state.
	deadline := time.Now().Add(30 * time.Second)
	var state status.State
	for {
		state, err = Status(id)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if state == status.Success {
			t.Fatalf("run succeeded, want failure")
		}
		if state == status.Error {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("run did not reach the error state in time (last: %s)", state)
		}
		time.Sleep(500 * time.Millisecond)
	}

	d, err := Detail(id)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if d.State != status.Error {
		t.Fatalf("detail state: got %s, want %s", d.State, status.Error)
	}
	if d.CreatedAt.IsZero() {
		t.Fatal("detail created_at: got zero, want the run's created_at")
	}
	if d.Attempts < 1 {
		t.Fatalf("detail attempts: got %d, want >= 1", d.Attempts)
	}
	if !strings.Contains(d.Error, "boom: step failed") {
		t.Fatalf("detail error: got %q, want the step error", d.Error)
	}
	if d.FailedStep != "boom-step" {
		t.Fatalf("detail failed step: got %q, want %q", d.FailedStep, "boom-step")
	}

	exs, err := Executions("failing-test")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(exs) != 1 || exs[0].ID != id || exs[0].State != status.Error {
		t.Fatalf("Executions: got %v, want the single failed run", exs)
	}
}

// The executed path of a dispatched branched run is the taken branch:
// the step rows in Postgres, step_id asc (phase 4, STEP 3).
func TestE2EStepsOfExecutedPath(t *testing.T) {
	dbURL := os.Getenv("DBOS_E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("DBOS_E2E_DATABASE_URL not set — skipping e2e test")
	}

	bootCoreForTest(t, dbURL)
	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	// Enqueue the branched workflow with a payload that takes the left
	// branch: the executed path is base → left.
	ex, err := Enqueue("decide", "left")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := ex.ID

	deadline := time.Now().Add(30 * time.Second)
	state, err := waitForTerminal(t, id, deadline)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if state != status.Success {
		t.Fatalf("run state: got %s, want %s", state, status.Success)
	}

	steps, err := Steps(id)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	want := []string{"base", "left"}
	if len(steps) != len(want) {
		t.Fatalf("Steps: got %v, want %v", steps, want)
	}
	for i, s := range steps {
		if s.Name != want[i] {
			t.Fatalf("Steps[%d]: got %q, want %q", i, s.Name, want[i])
		}
		if s.State != status.Success {
			t.Fatalf("Steps[%d]: got state %s, want %s", i, s.State, status.Success)
		}
	}
}

// A dispatched failing run: the executed path is the single failed step —
// the retries do not leave duplicate rows in the path (phase 4, STEP 3).
func TestE2EStepsOfFailedRun(t *testing.T) {
	dbURL := os.Getenv("DBOS_E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("DBOS_E2E_DATABASE_URL not set — skipping e2e test")
	}

	bootCoreForTest(t, dbURL)
	// A name of its own: the e2e database is persistent across runs, and
	// TestE2EDetailOfFailedRun asserts it sees exactly one failing-test run.
	registerTestWorkflow("failing-steps-test", failingTestWorkflow)
	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	ex, err := Enqueue("failing-steps-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := ex.ID

	deadline := time.Now().Add(30 * time.Second)
	state, err := waitForTerminal(t, id, deadline)
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if state != status.Error {
		t.Fatalf("run state: got %s, want %s", state, status.Error)
	}

	steps, err := Steps(id)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	if len(steps) != 1 || steps[0].Name != "boom-step" || steps[0].State != status.Error {
		t.Fatalf("Steps: got %v, want the single failed step", steps)
	}
}

// waitForTerminal polls Status until the run reaches success or error.
func waitForTerminal(t *testing.T, id string, deadline time.Time) (status.State, error) {
	t.Helper()
	for {
		state, err := Status(id)
		if err != nil {
			return "", err
		}
		if state == status.Success || state == status.Error {
			return state, nil
		}
		if time.Now().After(deadline) {
			return "", fmt.Errorf("run did not reach a terminal state in time (last: %s)", state)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

// Enqueue puts the run on the DBOS queue; the executor started by Launch
// picks it up and runs it to success (phase 2: INSERT INTO QUEUE → NOTIFY →
// steps).
func TestE2EEnqueueRunsWorkflow(t *testing.T) {
	dbURL := os.Getenv("DBOS_E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("DBOS_E2E_DATABASE_URL not set — skipping e2e test")
	}

	bootCoreForTest(t, dbURL)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	ex, err := Enqueue("hello-world", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if ex.ID == "" {
		t.Fatal("Enqueue returned an empty workflow ID")
	}
	if ex.CreatedAt.IsZero() {
		t.Fatal("Enqueue returned a zero CreatedAt")
	}
	id := ex.ID

	deadline := time.Now().Add(30 * time.Second)
	for {
		statuses, err := dbos.ListWorkflows(theCore.dbosCtx,
			dbos.WithFilterWorkflowIDs(id),
			dbos.WithFilterLoadInput(false),
			dbos.WithFilterLoadOutput(false),
		)
		if err != nil {
			t.Fatalf("ListWorkflows: %v", err)
		}
		if len(statuses) == 1 {
			switch statuses[0].Status {
			case dbos.WorkflowStatusSuccess:
				return
			case dbos.WorkflowStatusError:
				t.Fatalf("workflow failed: %v", statuses[0].Error)
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("workflow did not reach success in time: %v", statuses)
		}
		time.Sleep(500 * time.Millisecond)
	}
}
