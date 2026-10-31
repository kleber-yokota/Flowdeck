package workflows

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the branch workflow in the core DBOS context. With no env (unit
// tests, tools) the context is nil and the registration is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Branch,
			dbos.WithWorkflowName("branch"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// Branch is a workflow with a three-way if/else if/else: the graph forks the
// three branches side by side (left, mid, right).
func Branch(ctx dbos.Context, msg string) (string, error) {
	var r string
	if msg == "a" {
		_, err := dbos.RunAsStep(ctx, EchoStep("left"), dbos.WithStepName("left"))
		if err != nil {
			return "", err
		}
	} else if msg == "b" {
		_, err := dbos.RunAsStep(ctx, EchoStep("mid"), dbos.WithStepName("mid"))
		if err != nil {
			return "", err
		}

	} else {
		_, err := dbos.RunAsStep(ctx, EchoStep("right"), dbos.WithStepName("right"))
		if err != nil {
			return "", err
		}
	}
	r, err := dbos.RunAsStep(ctx, EchoStep("result"), dbos.WithStepName("result"))
	if err != nil {
		return "", err
	}
	return r, nil
}
