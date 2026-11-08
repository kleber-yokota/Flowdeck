package testworkflows

import (
	"context"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// linearStep is the echo step of the linear-test workflow (the shared
// echo step: inner-test and child-test reuse it).
func linearStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return msg, nil
	}
}

// linearTestWorkflow is the linear fixture: four steps in a row — a single
// possible path (fetch → parse → transform → store).
func linearTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	fetch, err := dbos.RunAsStep(ctx, linearStep("fetch"), dbos.WithStepName("fetch"))
	if err != nil {
		return "", err
	}
	parse, err := dbos.RunAsStep(ctx, linearStep("parse"), dbos.WithStepName("parse"))
	if err != nil {
		return "", err
	}
	transform, err := dbos.RunAsStep(ctx, linearStep("transform"), dbos.WithStepName("transform"))
	if err != nil {
		return "", err
	}
	store, err := dbos.RunAsStep(ctx, linearStep("store"), dbos.WithStepName("store"))
	if err != nil {
		return "", err
	}
	return fetch + " " + parse + " " + transform + " " + store, nil
}
