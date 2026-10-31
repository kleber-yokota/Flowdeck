package workflows

import (
	"context"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the fanout workflow in the core DBOS context. With no env (unit
// tests, tools) the context is nil and the registration is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Fanout,
			dbos.WithWorkflowName("fanout"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
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
