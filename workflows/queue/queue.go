// Package queue registers the queue workflow in the core DBOS context
// (the subpackage form: its own folder, its own Register).
package queue

import (
	"context"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// Register registers the queue workflow on the core DBOS context — the
// generated RegisterAll (workflows/zz_generated.go) calls it after
// the explicit boot.
func Register(c dbos.Context) {
	dbos.RegisterWorkflow(c, Queue,
		dbos.WithWorkflowName("queue"),
		dbos.WithMaxRecoveryAttempts(3),
	)
}

// queueDuration is the fixed sleep of the queue step (5s): long enough
// that consecutive dispatches pile up on the serial queue (visible on
// the web's /queued page).
const queueDuration = 5 * time.Second

// queueStep is the no-input step: a plain named function (no lambda —
// there is nothing to capture).
func queueStep(ctx context.Context) (string, error) {
	time.Sleep(queueDuration)
	return "slept", nil
}

// Queue is a single-step workflow: sleep for a fixed duration — it runs
// one at a time (the serial queue, concurrency 1), so consecutive
// dispatches queue up on it. The Go SDK requires the workflow's input
// parameter; the blank parameter is not an input (the parser skips it,
// the dispatch passes nil).
func Queue(ctx dbos.Context, _ string) (string, error) {
	out, err := dbos.RunAsStep(ctx, queueStep, dbos.WithStepName("sleep"))
	if err != nil {
		return "", err
	}
	return out, nil
}
