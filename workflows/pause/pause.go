package pause

import (
	"context"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core"
)

// Registers the pause workflow in the core DBOS context — the context
// is up since core's init (dependency order: core before this package).
// With no env (unit tests, tools) the context is nil and the registration
// is skipped.
func init() {
	if c := core.Ctx(); c != nil {
		dbos.RegisterWorkflow(c, Pause,
			dbos.WithWorkflowName("pause"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	}
}

// pauseDuration is the fixed pause of the pause step — long enough that a
// test can cancel the run mid-step and resume it (the resume tests).
const pauseDuration = 5 * time.Second

// PauseStep returns a step that pauses for pauseDuration and echoes the
// given message back.
func PauseStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		time.Sleep(pauseDuration)
		return "paused: " + msg, nil
	}
}

// Pause is a single-step workflow: pause for a fixed duration and echo the
// input back — the long run the resume tests cancel and re-run with the
// same stored input.
func Pause(ctx dbos.Context, msg string) (string, error) {
	out, err := dbos.RunAsStep(ctx, PauseStep(msg), dbos.WithStepName("pause"))
	if err != nil {
		return "", err
	}
	return out, nil
}
