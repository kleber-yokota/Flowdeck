package testworkflows

import (
	"context"
	"errors"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// liveTestWorkflow is the live-read seam: the payload picks the run
// outcome — "err" fails in the "live" step (retries run out: the row is
// ERROR), "block" blocks until the run is cancelled (the deterministic
// Running state the live tests cancel), any other payload succeeds.
func liveTestWorkflow(ctx dbos.Context, mode string) (string, error) {
	id, err := dbos.GetWorkflowID(ctx)
	if err != nil {
		return "", err
	}
	return dbos.RunAsStep(ctx, liveStep(mode, id, ctx), dbos.WithStepName("live"))
}

// liveStep is the step that picks its outcome by the stored payload.
func liveStep(mode, id string, env dbos.Context) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		switch mode {
		case "err":
			return "", errors.New("live: step failed")
		case "block":
			return blockUntilCancelled(env, id)
		default:
			return "ok", nil
		}
	}
}
