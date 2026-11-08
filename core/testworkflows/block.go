package testworkflows

import (
	"context"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// blockPollInterval is the poll of the block step over its own row (the
// step blocks until the run is cancelled — no fixed sleep window).
const blockPollInterval = 100 * time.Millisecond

// blockUntilCancelled blocks the caller until the run is cancelled: it
// polls the run's own row (the workflow id captured at the top of the
// workflow — constant per run, replay-safe) and returns as soon as the row
// is CANCELLED. The cancel tests get a deterministic Running state without
// a timing window, and the caller unblocks at the cancel (no leftover tail:
// the suite settles, the executor worker is released). The row stays
// CANCELLED (terminal) no matter what the workflow does after the cancel.
func blockUntilCancelled(env dbos.Context, id string) (string, error) {
	for {
		rows, err := dbos.ListWorkflows(env,
			dbos.WithFilterWorkflowIDs(id),
			dbos.WithFilterLoadInput(false),
			dbos.WithFilterLoadOutput(false),
		)
		if err == nil {
			for _, r := range rows {
				if r.Status == dbos.WorkflowStatusCancelled {
					return "cancelled", nil
				}
			}
		}
		time.Sleep(blockPollInterval)
	}
}

// blockStep is the block-test step: the closure of blockUntilCancelled on
// the workflow's env context.
func blockStep(id string, env dbos.Context) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return blockUntilCancelled(env, id)
	}
}

// blockTestWorkflow is the deterministic Running fixture: one step that
// blocks until the run is cancelled. The cancel tests own the run: they
// cancel it and the step unblocks at the cancel. A resume of a cancelled
// block run blocks again (the step has no recorded result at the cancel) —
// the resume tests use noinput-test instead.
func blockTestWorkflow(ctx dbos.Context, _ string) (string, error) {
	id, err := dbos.GetWorkflowID(ctx)
	if err != nil {
		return "", err
	}
	return dbos.RunAsStep(ctx, blockStep(id, ctx), dbos.WithStepName("block"))
}
