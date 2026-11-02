package testcore

import (
	"context"
	"errors"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// RegisterTestWorkflows registers the test-only workflows (failing-test ·
// resume-test) on the env context: registered here, never part of the
// workflows package, so they do not appear in the production registry.
func RegisterTestWorkflows(c dbos.Context) {
	dbos.RegisterWorkflow(c, failingTestWorkflow,
		dbos.WithWorkflowName("failing-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, flakyTestWorkflow,
		dbos.WithWorkflowName("resume-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// FlakyFail makes the flaky test workflow fail in its step — the test-only
// resume setup: a failed run with a known stored input.
var FlakyFail bool

// failingTestWorkflow is a workflow that fails in a named step — it
// exercises the failure path (retries, Detail: error + the failed step).
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

// flakyTestWorkflow fails in the "flaky" step while FlakyFail is set, and
// returns msg otherwise.
func flakyTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	return dbos.RunAsStep(ctx, flakyStep(msg), dbos.WithStepName("flaky"))
}

// flakyStep is the step that fails while FlakyFail is set.
func flakyStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		if FlakyFail {
			return "", errors.New("flaky: step failed")
		}
		return msg, nil
	}
}
