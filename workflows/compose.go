package workflows

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the compose workflow in the core DBOS context. With no env (unit
// tests, tools) the context is nil and the registration is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Compose,
			dbos.WithWorkflowName("compose"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// Compose is a workflow with a child workflow (RunWorkflow): it runs the
// pipeline as a child, then stamps the result. The graph shows the child
// node (Pipeline) with the child symbol.
func Compose(ctx dbos.Context, msg string) (string, error) {
	handle, err := dbos.RunWorkflow(ctx, Pipeline, msg)
	if err != nil {
		return "", err
	}
	childResult, err := handle.GetResult()
	if err != nil {
		return "", err
	}
	stamp, err := dbos.RunAsStep(ctx, EchoStep("stamped "+childResult), dbos.WithStepName("stamp"))
	if err != nil {
		return "", err
	}
	return stamp, nil
}
