// Package hello registers the hello-world workflow in the core DBOS context
// (the subpackage form: its own folder, its own Register).
package hello

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/workflows/steps"
)

// defaultMsg is the default value of the hello-world workflow's msg input
// (the parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Register registers the hello workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, HelloWorld,
		dbos.WithWorkflowName("hello-world"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// HelloWorld is an example workflow with a durable step.
func HelloWorld(ctx dbos.Context, msg string) (string, error) {
	greeting, err := dbos.RunAsStep(ctx, steps.GreetStep(msg), dbos.WithStepName("greet"))
	if err != nil {
		return "", err
	}
	return greeting, nil
}
