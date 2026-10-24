package core

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

func init() {
	register("pipeline",
		func(ctx dbos.Context) {
			dbos.RegisterWorkflow(ctx, Pipeline,
				dbos.WithWorkflowName("pipeline"),
				dbos.WithMaxRecoveryAttempts(3),
			)
		},
		Pipeline,
	)
}

// Pipeline is a workflow with linear steps: four steps in a row — a single
// possible path (fetch → parse → transform → store).
func Pipeline(ctx dbos.Context, msg string) (string, error) {
	fetch, err := dbos.RunAsStep(ctx, EchoStep("fetch"), dbos.WithStepName("fetch"))
	if err != nil {
		return "", err
	}
	parse, err := dbos.RunAsStep(ctx, EchoStep("parse"), dbos.WithStepName("parse"))
	if err != nil {
		return "", err
	}
	transform, err := dbos.RunAsStep(ctx, EchoStep("transform"), dbos.WithStepName("transform"))
	if err != nil {
		return "", err
	}
	store, err := dbos.RunAsStep(ctx, EchoStep("store"), dbos.WithStepName("store"))
	if err != nil {
		return "", err
	}
	return fetch + " " + parse + " " + transform + " " + store, nil
}
