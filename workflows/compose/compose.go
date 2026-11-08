// Package compose registers the compose workflow in the core DBOS context
// (the subpackage form: its own folder, its own Register).
package compose

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/workflows/pipeline"
	"flowdeck/workflows/steps"
)

// defaultMsg is the default value of the compose workflow's msg input (the
// parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Register registers the compose workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Compose,
		dbos.WithWorkflowName("compose"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// Compose is a workflow with a child workflow (RunWorkflow): it runs the
// pipeline as a child, then stamps the result. The graph shows the child
// node (Pipeline) with the child symbol.
func Compose(ctx dbos.Context, msg string) (string, error) {
	handle, err := dbos.RunWorkflow(ctx, pipeline.Pipeline, msg)
	if err != nil {
		return "", err
	}
	childResult, err := handle.GetResult()
	if err != nil {
		return "", err
	}
	stamp, err := dbos.RunAsStep(ctx, steps.EchoStep("stamped "+childResult), dbos.WithStepName("stamp"))
	if err != nil {
		return "", err
	}
	return stamp, nil
}
