// Package pipeline registers the pipeline workflow in the core DBOS context
// (the subpackage form: its own folder, its own Register).
package pipeline

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/workflows/steps"
)

// defaultMsg is the default value of the pipeline workflow's msg input
// (the parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Register registers the pipeline workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Pipeline,
		dbos.WithWorkflowName("pipeline"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// Pipeline is a workflow with linear steps: four steps in a row — a single
// possible path (fetch → parse → transform → store).
func Pipeline(ctx dbos.Context, msg string) (string, error) {
	fetch, err := dbos.RunAsStep(ctx, steps.EchoStep("fetch"), dbos.WithStepName("fetch"))
	if err != nil {
		return "", err
	}
	parse, err := dbos.RunAsStep(ctx, steps.EchoStep("parse"), dbos.WithStepName("parse"))
	if err != nil {
		return "", err
	}
	transform, err := dbos.RunAsStep(ctx, steps.EchoStep("transform"), dbos.WithStepName("transform"))
	if err != nil {
		return "", err
	}
	store, err := dbos.RunAsStep(ctx, steps.EchoStep("store"), dbos.WithStepName("store"))
	if err != nil {
		return "", err
	}
	return fetch + " " + parse + " " + transform + " " + store, nil
}
