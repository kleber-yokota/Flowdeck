package testworkflows

import (
	"context"
	"errors"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

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
