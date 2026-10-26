package workflows

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the schedule workflow in the core DBOS context — the context
// is up since core's init (dependency order: core before this package).
// With no env (unit tests, tools) the context is nil and the registration
// is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Schedule,
			dbos.WithWorkflowName("schedule"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// ScheduleArgs is the schedule workflow's input (phase 5: one form item
// per field, from the spec the core parses at boot).
type ScheduleArgs struct {
	Name      string
	StartDate string
}

// The default values of the schedule's inputs (phase 5, BOOT: the parser
// extracts the const default<NamePascal> of the package).
const (
	defaultName      = "FlowDeck"
	defaultStartDate = "2026-01-01"
)

// Schedule is a workflow with a struct input: one step that echoes what
// was scheduled.
func Schedule(ctx dbos.Context, args ScheduleArgs) (string, error) {
	scheduled, err := dbos.RunAsStep(ctx, EchoStep("scheduled "+args.Name+" @ "+args.StartDate), dbos.WithStepName("schedule"))
	if err != nil {
		return "", err
	}
	return scheduled, nil
}
