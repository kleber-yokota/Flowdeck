package workflows

import (
	"context"
	"strings"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the kitchen workflow in the core DBOS context. With no env (unit
// tests, tools) the context is nil and the registration is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Kitchen,
			dbos.WithWorkflowName("kitchen"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// Kitchen is a workflow with every node kind: a step, a sleep (wait), a
// child (RunWorkflow), an async (Go), a select (wait), and a loop. The graph
// shows the vertical stack with the kind symbols.
func Kitchen(ctx dbos.Context, msg string) (string, error) {
	base, err := dbos.RunAsStep(ctx, EchoStep("base"), dbos.WithStepName("base"))
	if err != nil {
		return "", err
	}
	_, err = dbos.Sleep(ctx, time.Second)
	if err != nil {
		return "", err
	}
	handle, err := dbos.RunWorkflow(ctx, Pipeline, msg)
	if err != nil {
		return "", err
	}
	child, err := handle.GetResult()
	if err != nil {
		return "", err
	}
	side, err := dbos.Go(ctx, func(_ context.Context) (string, error) {
		return "side", nil
	}, dbos.WithStepName("side"))
	if err != nil {
		return "", err
	}
	sel, err := dbos.Select(ctx, []<-chan dbos.StepOutcome[string]{side})
	if err != nil {
		return "", err
	}
	var out []string
	for i := 0; i < 2; i++ {
		spin, err := dbos.RunAsStep(ctx, EchoStep("spin"), dbos.WithStepName("spin"))
		if err != nil {
			return "", err
		}
		out = append(out, spin)
	}
	return base + " " + child + " " + sel + " " + strings.Join(out, " "), nil
}
