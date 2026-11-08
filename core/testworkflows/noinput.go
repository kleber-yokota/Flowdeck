package testworkflows

import (
	"context"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// noinputTestStep is the no-input step: a plain named function (there is
// nothing to capture) — the dispatch passes nil, the parser skips the blank
// parameter.
func noinputTestStep(_ context.Context) (string, error) {
	return "ok", nil
}

// noinputTestWorkflow is the no-input workflow: no form, one Run button,
// the run reaches success fast (the dispatch and the resume tests).
func noinputTestWorkflow(ctx dbos.Context, _ string) (string, error) {
	return dbos.RunAsStep(ctx, noinputTestStep, dbos.WithStepName("noinput"))
}
