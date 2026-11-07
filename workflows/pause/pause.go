package pause

import (
	"context"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// Register registers the pause workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Pause,
		dbos.WithWorkflowName("pause"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// pauseDuration is the fixed pause of the pause step — long enough that a
// test can cancel the run mid-step and resume it (the resume tests).
const pauseDuration = 10 * time.Second

// pauseStep is the no-input step: a plain named function (no lambda —
// there is nothing to capture).
func pauseStep(ctx context.Context) (string, error) {
	time.Sleep(pauseDuration)
	return "paused", nil
}

// Pause is a single-step workflow: pause for a fixed duration. The Go SDK
// requires the workflow's input parameter; the blank parameter is not an
// input (the parser skips it, the dispatch passes nil).
func Pause(ctx dbos.Context, _ string) (string, error) {
	out, err := dbos.RunAsStep(ctx, pauseStep, dbos.WithStepName("pause"))
	if err != nil {
		return "", err
	}
	return out, nil
}
