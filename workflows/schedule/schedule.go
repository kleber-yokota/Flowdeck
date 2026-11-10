// Package schedule registers the schedule workflow in the core DBOS context
// (the subpackage form: its own folder, its own Register).
package schedule

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/workflows/steps"
)

// Register registers the schedule workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Schedule,
		dbos.WithWorkflowName("schedule"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// ScheduleArgs is the schedule workflow's input (one form item
// per field, from the spec the core parses at boot).
type ScheduleArgs struct {
	Name      string
	StartDate string
}

// The default values of the schedule's inputs (the parser
// extracts the const default<NamePascal> of the package).
const (
	defaultName      = "FlowDeck"
	defaultStartDate = "2026-01-01"
)

// Schedule is a workflow with a struct input: one step that echoes what
// was scheduled.
func Schedule(ctx dbos.Context, args ScheduleArgs) (string, error) {
	scheduled, err := dbos.RunAsStep(ctx, steps.EchoStep("scheduled "+args.Name+" @ "+args.StartDate), dbos.WithStepName("schedule"))
	if err != nil {
		return "", err
	}
	return scheduled, nil
}
