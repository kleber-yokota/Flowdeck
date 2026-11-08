// Package branch registers the branch workflow in the core DBOS context
// (the subpackage form: its own folder, its own Register).
package branch

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/workflows/steps"
)

// defaultMsg is the default value of the branch workflow's msg input (the
// parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Register registers the branch workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Branch,
		dbos.WithWorkflowName("branch"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// Branch is a workflow with a three-way if/else if/else: the graph forks the
// three branches side by side (left, mid, right).
func Branch(ctx dbos.Context, msg string) (string, error) {
	var r string
	if msg == "a" {
		_, err := dbos.RunAsStep(ctx, steps.EchoStep("left"), dbos.WithStepName("left"))
		if err != nil {
			return "", err
		}
	} else if msg == "b" {
		_, err := dbos.RunAsStep(ctx, steps.EchoStep("mid"), dbos.WithStepName("mid"))
		if err != nil {
			return "", err
		}

	} else {
		_, err := dbos.RunAsStep(ctx, steps.EchoStep("right"), dbos.WithStepName("right"))
		if err != nil {
			return "", err
		}
	}
	r, err := dbos.RunAsStep(ctx, steps.EchoStep("result"), dbos.WithStepName("result"))
	if err != nil {
		return "", err
	}
	return r, nil
}
