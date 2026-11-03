// Package hello registers the hello-world workflow in the core DBOS context
// (the subpackage form: its own folder, its own init).
package hello

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/workflows/steps"
)

// defaultMsg is the default value of the hello-world workflow's msg input
// (the parser extracts the const default<NamePascal>).
const defaultMsg = "FlowDeck"

// Registers the hello-world workflow in the core DBOS context — the
// context is up since boot's init (dependency order: core before this
// package). With no env (unit tests, tools) the context is nil and the
// registration is skipped.
func init() {
	if c := boot.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, HelloWorld,
			dbos.WithWorkflowName("hello-world"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// HelloWorld is an example workflow with a durable step.
func HelloWorld(ctx dbos.Context, msg string) (string, error) {
	greeting, err := dbos.RunAsStep(ctx, steps.GreetStep(msg), dbos.WithStepName("greet"))
	if err != nil {
		return "", err
	}
	return greeting, nil
}
