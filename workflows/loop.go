package workflows

import (
	"strings"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the loop workflow in the core DBOS context. With no env (unit
// tests, tools) the context is nil and the registration is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Loop,
			dbos.WithWorkflowName("loop"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// Loop is a workflow with a loop block: a for-range that repeats a step.
// The graph shows the loop outline (L1) with the spin step inside.
func Loop(ctx dbos.Context, msg string) (string, error) {
	var out []string
	for _, item := range []string{"x", "y", "z"} {
		spin, err := dbos.RunAsStep(ctx, EchoStep("spin "+item), dbos.WithStepName("spin"))
		if err != nil {
			return "", err
		}
		out = append(out, spin)
	}
	return strings.Join(out, " "), nil
}
