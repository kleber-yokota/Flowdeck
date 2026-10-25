package core

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// Each workflow file registers itself in its own init() — there is no
// central list. A new workflow = a new file in this package with its init().
func init() {
	register("hello-world",
		func(ctx dbos.Context) {
			dbos.RegisterWorkflow(ctx, HelloWorld,
				dbos.WithWorkflowName("hello-world"),
				dbos.WithMaxRecoveryAttempts(3),
			)
		},
		HelloWorld,
	)
}

// HelloWorld is an example workflow with a durable step.
func HelloWorld(ctx dbos.Context, msg string) (string, error) {
	greeting, err := dbos.RunAsStep(ctx, GreetStep(msg), dbos.WithStepName("greet"))
	if err != nil {
		return "", err
	}
	return greeting, nil
}
