package testworkflows

import (
	"context"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// forkStep is the echo step of the fork-test workflow (the branch is
// chosen by the payload at the workflow level, not in the step).
func forkStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return msg, nil
	}
}

// forkTestWorkflow is the branch fixture: after the base step, the second
// step depends on the payload — two possible paths (base → left and base →
// right): the if the parser forks on.
func forkTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	base, err := dbos.RunAsStep(ctx, forkStep("base"), dbos.WithStepName("base"))
	if err != nil {
		return "", err
	}
	var choice string
	if msg != "" {
		left, err := dbos.RunAsStep(ctx, forkStep("left"), dbos.WithStepName("left"))
		if err != nil {
			return "", err
		}
		choice = left
	} else {
		right, err := dbos.RunAsStep(ctx, forkStep("right"), dbos.WithStepName("right"))
		if err != nil {
			return "", err
		}
		choice = right
	}
	return base + " " + choice, nil
}
