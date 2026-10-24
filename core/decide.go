package core

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

func init() {
	register("decide",
		func(ctx dbos.Context) {
			dbos.RegisterWorkflow(ctx, Decide,
				dbos.WithWorkflowName("decide"),
				dbos.WithMaxRecoveryAttempts(3),
			)
		},
		Decide,
	)
}

// Decide is a workflow with a branch: after the first step, the second step
// depends on the payload — two possible paths (base → left and base →
// right). The if/else is what the boot parser (phase 4, STEP 1) forks on.
func Decide(ctx dbos.Context, msg string) (string, error) {
	base, err := dbos.RunAsStep(ctx, EchoStep("base"), dbos.WithStepName("base"))
	if err != nil {
		return "", err
	}
	var choice string
	if msg != "" {
		left, err := dbos.RunAsStep(ctx, EchoStep("left"), dbos.WithStepName("left"))
		if err != nil {
			return "", err
		}
		choice = left
	} else {
		right, err := dbos.RunAsStep(ctx, EchoStep("right"), dbos.WithStepName("right"))
		if err != nil {
			return "", err
		}
		choice = right
	}
	return base + " " + choice, nil
}
