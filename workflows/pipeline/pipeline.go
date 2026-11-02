// Package pipeline registers the pipeline workflow in the core DBOS context
// (the subpackage form: its own folder, its own init).
package pipeline

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
	"flowdeck/workflows/steps"
)

// defaultMsg is the default value of the pipeline workflow's msg input
// (the parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Registers the pipeline workflow in the core DBOS context — the context
// is up since core's init (dependency order: core before this package).
// With no env (unit tests, tools) the context is nil and the registration
// is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Pipeline,
			dbos.WithWorkflowName("pipeline"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
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
