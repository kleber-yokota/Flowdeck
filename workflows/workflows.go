// Package workflows holds the durable workflows. Each file registers its
// workflow in the core DBOS context (core.Ctx()) in init — see the
// registration comment below.
//
// New workflow subpackage (workflows/<name>/): add the files, then run
//
//	go generate ./workflows
//
// which rewrites zz_generated.go (the blank imports that pull the
// subpackages into the binary). main.go never changes for a new workflow.
//
package workflows

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the hello-world workflow in the core DBOS context — the
// context is up since core's init (dependency order: core before this
// package). With no env (unit tests, tools) the context is nil and the
// registration is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, HelloWorld,
			dbos.WithWorkflowName("hello-world"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// HelloWorld is an example workflow with a durable step.
func HelloWorld(ctx dbos.Context, msg string) (string, error) {
	greeting, err := dbos.RunAsStep(ctx, GreetStep(msg), dbos.WithStepName("greet"))
	if err != nil {
		return "", err
	}
	return greeting, nil
}
