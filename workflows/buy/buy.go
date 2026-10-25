// Package buy registers the buy workflow in the core DBOS context — the
// subpackage form: its own file, its own init. The source is embedded in
// main (the whole workflows tree), so the paths parser finds it for free.
package buy

import (
	"context"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the buy workflow in the core DBOS context — the context is up
// since core's init (dependency order: core before this package). With no
// env (unit tests, tools) the context is nil and the registration is
// skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Buy,
			dbos.WithWorkflowName("buy"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// ChargeStep returns a step that "charges" the given message — the plain
// durable step behind the buy workflow.
func ChargeStep(msg string) dbos.Step[string] {
	return func(ctx context.Context) (string, error) {
		return "charged: " + msg, nil
	}
}

// Buy is a single-step workflow (charge).
func Buy(ctx dbos.Context, msg string) (string, error) {
	charged, err := dbos.RunAsStep(ctx, ChargeStep(msg), dbos.WithStepName("charge"))
	if err != nil {
		return "", err
	}
	return charged, nil
}
