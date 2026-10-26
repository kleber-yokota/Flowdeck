package core_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
	"flowdeck/core/graph"
	"flowdeck/core/status"

	// The workflow files register themselves in the env-driven core DBOS
	// context (their init) — the same single blank import main.go carries;
	// workflows/zz_generated.go pulls in every subpackage.
	_ "flowdeck/workflows"
)

// The core tests use the env-driven lifecycle: core.init() has already
// created the DBOS context from DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL
// (see .env.test), and the workflow files (which import core) have
// registered their workflows in their own init. This TestMain only
// registers the test-only failing workflow and starts the executor once.
func TestMain(m *testing.M) {
	core.SetWorkflowsSource(os.DirFS("../workflows"))
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, failingTestWorkflow,
			dbos.WithWorkflowName("failing-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
		if err := core.Launch(); err != nil {
			fmt.Fprintln(os.Stderr, "core_test: launch:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// needCore skips the test when the env-driven context is absent (the env
// vars were not exported before the test binary started).
func needCore(t *testing.T) {
	t.Helper()
	if core.Ctx() == nil {
		t.Skip("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — source .env.test")
	}
}

// failingTestWorkflow is a workflow that fails in a named step — it
// exercises the failure path (retries, Detail: error + the failed step).
// Test-only: registered here by the TestMain, never part of the workflows
// package, so it does not appear in the production registry.
func failingTestWorkflow(ctx dbos.Context, _ string) (string, error) {
	_, err := dbos.RunAsStep(ctx, failingTestStep(), dbos.WithStepName("boom-step"))
	if err != nil {
		return "", err
	}
	return "", nil
}

// failingTestStep is a step that always fails.
func failingTestStep() dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return "", errors.New("boom: step failed")
	}
}

// pollState polls the badge state until the wanted one (a success stops the
// poll too — the run finished, just not with the wanted state).
func pollState(t *testing.T, id string, want status.State) (status.State, error) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		st, err := core.Status(id)
		if err != nil {
			return st, fmt.Errorf("Status: %w", err)
		}
		if st == want || st == status.Success {
			return st, nil
		}
		if time.Now().After(deadline) {
			return st, fmt.Errorf("poll deadline, last state %s", st)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// Phase 5, BOOT: the core exposes the spec of a registered workflow —
// the input and the steps' default values, parsed from the source at boot
// (the web renders them on top of the workflow page).
func TestSpecOfRegisteredWorkflow(t *testing.T) {
	spec, err := core.Spec("decide")
	if err != nil {
		t.Fatalf("core.Spec(decide): %v", err)
	}
	if spec.Input.Name != "msg" || spec.Input.Type != "string" {
		t.Fatalf("decide input: got %+v, want {msg string}", spec.Input)
	}
	wantSteps := map[string]string{"base": "base", "left": "left", "right": "right"}
	if len(spec.Steps) != len(wantSteps) {
		t.Fatalf("decide steps: got %v, want %v", spec.Steps, wantSteps)
	}
	for _, s := range spec.Steps {
		if wantSteps[s.Name] != s.Default {
			t.Fatalf("decide step %s: got default %q, want %q", s.Name, s.Default, wantSteps[s.Name])
		}
	}
}

// Spec of a name that is not in the source errors with ErrUnknownWorkflow
// (the web maps it to a 404).
func TestSpecOfUnknownWorkflow(t *testing.T) {
	if _, err := core.Spec("no-such-workflow"); !errors.Is(err, core.ErrUnknownWorkflow) {
		t.Fatalf("Spec unknown: got %v, want ErrUnknownWorkflow", err)
	}
}

// The registry (the web listing's source) has the shipped workflows — the
// registration is the workflow files' init, applied to the env context.
func TestListShowsRegisteredWorkflows(t *testing.T) {
	needCore(t)
	names := map[string]bool{}
	for _, w := range core.List() {
		names[w.Name] = true
	}
	for _, want := range []string{"hello-world", "pipeline", "decide", "buy", "pause"} {
		if !names[want] {
			t.Fatalf("registry missing %s: %v", want, names)
		}
	}
}

// Enqueue (the dispatch) returns the new execution (id + state +
// created_at) — the dispatch ACK the web renders as the "queued" row.
func TestEnqueueReturnsExecution(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("hello-world", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if ex.ID == "" || ex.ID == "00000000-0000-0000-0000-000000000000" {
		t.Fatalf("ACK without an execution id: %+v", ex)
	}
	if ex.State != status.Queued && ex.State != status.Running {
		t.Fatalf("ACK state: got %s, want queued/running", ex.State)
	}
	if ex.CreatedAt.IsZero() {
		t.Fatal("ACK without a created_at")
	}
}

// Enqueue of a name that is not in the registry errors with
// ErrUnknownWorkflow (the web maps it to a 400).
func TestEnqueueUnknownWorkflow(t *testing.T) {
	needCore(t)
	if _, err := core.Enqueue("no-such-workflow", nil); !errors.Is(err, core.ErrUnknownWorkflow) {
		t.Fatalf("Enqueue unknown: got %v, want ErrUnknownWorkflow", err)
	}
}

// Phase 5, EXECUTE: the dispatch sanitizes the raw form values against
// the spec (the core) and enqueues the clean input — the run takes the
// value (the decide workflow branches on it: base + left, no right).
func TestEnqueueWithValuesTakesBranch(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("decide", map[string]string{"msg": "left"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	for _, name := range mustStepNames(t, ex) {
		if name == "right" {
			t.Fatalf("steps: right ran, want the left branch: %v", mustStepNames(t, ex))
		}
	}
}

// A form value that is not in the spec is rejected (the web maps
// ErrInvalidInput to a 400).
func TestEnqueueUnknownField(t *testing.T) {
	needCore(t)
	if _, err := core.Enqueue("decide", map[string]string{"msg": "x", "bogus": "y"}); !errors.Is(err, core.ErrInvalidInput) {
		t.Fatalf("Enqueue: got %v, want ErrInvalidInput", err)
	}
}

// A run that fails in a step reaches the failed state, and Detail shows the
// error and the failed step (phase 3, DETAIL: on failure: error + the failed
// step).
func TestFailingRunDetailHasErrorAndFailedStep(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("failing-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	st, err := pollState(t, ex.ID, status.Error)
	if err != nil {
		t.Fatalf("poll to error: %v", err)
	}
	if st != status.Error {
		t.Fatalf("failing run state: got %s, want error", st)
	}
	d, err := core.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if !strings.Contains(d.Error, "boom: step failed") {
		t.Fatalf("Detail error: got %q", d.Error)
	}
	if d.FailedStep != "boom-step" {
		t.Fatalf("Detail failed step: got %q, want boom-step", d.FailedStep)
	}
	if d.Attempts < 1 {
		t.Fatalf("Detail attempts: %d, want >= 1", d.Attempts)
	}
}

// mustStepNames returns the step names of the run, failing the test if the
// steps cannot be read.
func mustStepNames(t *testing.T, ex core.Execution) []string {
	t.Helper()
	steps, err := core.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Name)
	}
	return names
}

// Executions lists the runs of a workflow (phase 3, LIST), newest first:
// the run I enqueued is in the list, with its final state (the assertions
// filter by id — the test database may hold runs from other tests).
func TestExecutionsListsRunsOfWorkflow(t *testing.T) {
	needCore(t)
	ex, err := core.Enqueue("hello-world", map[string]string{"msg": "one"})
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	list, err := core.Executions("hello-world")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	found := false
	for _, e := range list {
		if e.ID != ex.ID {
			continue
		}
		found = true
		if e.State != status.Success {
			t.Fatalf("execution %s state: got %s, want success", e.ID, e.State)
		}
	}
	if !found {
		t.Fatalf("run %s not in Executions(hello-world)", ex.ID)
	}
}

// Executions of a name with no runs is empty (no error).
func TestExecutionsUnknownWorkflowIsEmpty(t *testing.T) {
	needCore(t)
	list, err := core.Executions("no-such-workflow")
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("Executions(unknown): got %d rows, want 0", len(list))
	}
}

// The reads of an execution that is not known by the core error with
// ErrUnknownExecution (the web maps it to a 404).
func TestDetailAndStepsOfUnknownExecution(t *testing.T) {
	needCore(t)
	unknown := "00000000-0000-0000-0000-000000000000"
	if _, err := core.Detail(unknown); !errors.Is(err, core.ErrUnknownExecution) {
		t.Fatalf("Detail unknown: got %v, want ErrUnknownExecution", err)
	}
	if _, err := core.Steps(unknown); !errors.Is(err, core.ErrUnknownExecution) {
		t.Fatalf("Steps unknown: got %v, want ErrUnknownExecution", err)
	}
	if _, err := core.Status(unknown); !errors.Is(err, core.ErrUnknownExecution) {
		t.Fatalf("Status unknown: got %v, want ErrUnknownExecution", err)
	}
}

// The embedded source (parsed, no imports involved) is the ground truth
// for which workflows exist: every workflow it declares must be registered
// in the core DBOS. A mismatch means workflows/zz_generated.go is stale —
// a new subpackage was added without running: go generate ./workflows.
func TestRegistrationMatchesSource(t *testing.T) {
	needCore(t)
	g, err := graph.New(os.DirFS("../workflows"))
	if err != nil {
		t.Fatalf("build graph from source: %v", err)
	}
	registered := map[string]bool{}
	for _, w := range core.List() {
		registered[w.Name] = true
	}
	for name := range g.ByName {
		if !registered[name] {
			t.Fatalf("source declares workflow %q but the registry does not — run 'go generate ./workflows'", name)
		}
	}
}
