package testworkflows

import (
	"context"
	"errors"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// FlakyFail makes the flaky test workflow fail in its step — the test-only
// resume setup: a failed run with a known stored input.
var FlakyFail bool

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
