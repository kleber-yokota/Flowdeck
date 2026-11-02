package core_test

import (
	"context"
	"errors"
	"flowdeck/core"
	"flowdeck/core/status"
	"fmt"
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
	"os"
	"testing"
	"time"

	// The workflow files register themselves in the env-driven core DBOS
	// context (their init) — the same single blank import main.go carries;
	// workflows/zz_generated.go pulls in every subpackage.
	_ "flowdeck/workflows"
)

// The core tests use the env-driven lifecycle: core.init() has already
// created the DBOS context from DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL
// (see .env.test), and the workflow files (which import core) have
// registered their workflows in their own init. This TestMain only
// registers the test-only failing and flaky workflows and starts the
// executor once.
func TestMain(m *testing.M) {
	core.SetWorkflowsSource(os.DirFS("../workflows"))
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, failingTestWorkflow,
			dbos.WithWorkflowName("failing-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
		dbos.RegisterWorkflow(c, flakyTestWorkflow,
			dbos.WithWorkflowName("resume-test"),
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

// flakyFail makes the flaky test workflow fail in its step — the test-only
// resume setup: a failed run with a known stored input.
var flakyFail bool

// flakyTestWorkflow fails in the "flaky" step while flakyFail is set, and
// returns msg otherwise. Test-only: registered here by the TestMain, never
// part of the workflows package, so it does not appear in the production
// registry (like failing-test).
func flakyTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	return dbos.RunAsStep(ctx, flakyStep(msg), dbos.WithStepName("flaky"))
}

// flakyStep is the step that fails while flakyFail is set.
func flakyStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		if flakyFail {
			return "", errors.New("flaky: step failed")
		}
		return msg, nil
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

// mustStepNames returns the step names of the run, failing the test if the
// steps cannot be read.
func mustStepNames(t *testing.T, ex core.Execution) []string {
	t.Helper()
	path, err := core.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	names := make([]string, 0, len(path.Steps))
	for _, s := range path.Steps {
		names = append(names, s.Name)
	}
	return names
}
