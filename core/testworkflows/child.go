package testworkflows

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// childTestWorkflow is the child-dispatch fixture: a base step and a child
// run (the Detail children seam — the child appears in the steps of the
// parent run).
func childTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	base, err := dbos.RunAsStep(ctx, linearStep("base"), dbos.WithStepName("base"))
	if err != nil {
		return "", err
	}
	handle, err := dbos.RunWorkflow(ctx, innerTestWorkflow, msg)
	if err != nil {
		return "", err
	}
	child, err := handle.GetResult()
	if err != nil {
		return "", err
	}
	return base + " " + child, nil
}
