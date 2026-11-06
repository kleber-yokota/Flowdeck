package testworkflows

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// innerTestWorkflow is the child fixture: a single-step workflow that
// child-test runs as a child run (the Detail children seam).
func innerTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	return dbos.RunAsStep(ctx, linearStep(msg), dbos.WithStepName("inner"))
}
