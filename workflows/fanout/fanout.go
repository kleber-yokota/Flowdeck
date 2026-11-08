// Package fanout registers the fanout workflow in the core DBOS context
// (the subpackage form: its own folder, its own Register).
package fanout

import (
	"context"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// defaultMsg is the default value of the fanout workflow's msg input (the
// parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Register registers the fanout workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Fanout,
		dbos.WithWorkflowName("fanout"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// Fanout is a workflow with async (Go) runs and a durable select: two async
// steps side by side, then a select that waits for the first result.
func Fanout(ctx dbos.Context, msg string) (string, error) {
	alpha, err := dbos.Go(ctx, func(_ context.Context) (string, error) {
		return "alpha", nil
	}, dbos.WithStepName("alpha"))
	if err != nil {
		return "", err
	}
	beta, err := dbos.Go(ctx, func(_ context.Context) (string, error) {
		return "beta", nil
	}, dbos.WithStepName("beta"))
	if err != nil {
		return "", err
	}
	chosen, err := dbos.Select(ctx, []<-chan dbos.StepOutcome[string]{alpha, beta})
	if err != nil {
		return "", err
	}
	return chosen, nil
}
