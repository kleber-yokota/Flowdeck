// Package decide registers the decide workflow in the core DBOS context
// (the subpackage form: its own folder, its own Register).
package decide

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/workflows/steps"
)

// defaultMsg is the default value of the decide workflow's msg input (the
// parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Register registers the decide workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Decide,
		dbos.WithWorkflowName("decide"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// Decide is a workflow with a branch: after the first step, the second step
// depends on the payload — two possible paths (base → left and base →
// right). The if/else is what the boot parser (phase 4, STEP 1) forks on.
func Decide(ctx dbos.Context, msg string) (string, error) {
	base, err := dbos.RunAsStep(ctx, steps.EchoStep("base"), dbos.WithStepName("base"))
	if err != nil {
		return "", err
	}
	var choice string
	if msg != "" {
		left, err := dbos.RunAsStep(ctx, steps.EchoStep("left"), dbos.WithStepName("left"))
		if err != nil {
			return "", err
		}
		choice = left
	} else {
		right, err := dbos.RunAsStep(ctx, steps.EchoStep("right"), dbos.WithStepName("right"))
		if err != nil {
			return "", err
		}
		choice = right
	}
	return base + " " + choice, nil
}
