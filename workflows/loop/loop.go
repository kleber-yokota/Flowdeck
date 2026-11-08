// Package loop registers the loop workflow in the core DBOS context (the
// subpackage form: its own folder, its own Register).
package loop

import (
	"strings"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/workflows/steps"
)

// defaultMsg is the default value of the loop workflow's msg input (the
// parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Register registers the loop workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Loop,
		dbos.WithWorkflowName("loop"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// Loop is a workflow with a loop block: a for-range that repeats a step.
// The graph shows the loop outline (L1) with the spin step inside.
func Loop(ctx dbos.Context, msg string) (string, error) {
	var out []string
	for _, item := range []string{"x", "y", "z"} {
		spin, err := dbos.RunAsStep(ctx, steps.EchoStep("spin "+item), dbos.WithStepName("spin"))
		if err != nil {
			return "", err
		}
		out = append(out, spin)
	}
	return strings.Join(out, " "), nil
}
