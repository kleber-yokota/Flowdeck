// Package actions is the actions on the executions of the core: the
// Enqueue (the dispatch — the run goes onto the DBOS queue), the Resume
// (a failed run re-enqueues by id) and the Cancel (a live run cancels by
// id). The actions act on runs; the reads observe them.
package actions

import (
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/registry"
)

// Enqueue puts a run of the named workflow onto the DBOS queue and returns
// the new execution (id + state + created_at) — the dispatch ACK: the web
// renders the "queued" row with it, and nothing is executed by the
// webserver.
// The payload is the clean input (the web sanitized it against the
// workflow's spec before the call — the core trusts the caller); a nil
// payload is the zero payload (a dispatch without a form — the SDK decodes
// the stored "null" back into the zero value at run time).
func Enqueue(name string, payload any) (reads.Execution, error) {
	c, err := boot.CtxOrErr()
	if err != nil {
		return reads.Execution{}, err
	}
	for _, w := range registry.List() {
		if w.Name == name {
			return dispatch(c, name, payload)
		}
	}
	return reads.Execution{}, fmt.Errorf("%w: %s", registry.ErrUnknownWorkflow, name)
}

// serial is the name → serial-queue map (the dispatch table): the
// workflows that must run one at a time — their dispatches and resumes go
// through the serial queue (concurrency 1, boot.SerialQueueName), the
// rest go through the main queue.
var serial = map[string]bool{
	"queue": true,
}

// queueNameFor picks the dispatch's queue by workflow name: the serial
// queue (concurrency 1) for the serial dispatches, the main queue for
// the rest.
func queueNameFor(name string) string {
	if serial[name] {
		return boot.SerialQueueName
	}
	return boot.QueueName
}

// dispatch puts payload on the queue under name and returns the new
// execution (id + state + created_at) — the dispatch ACK: the web renders
// the "queued" row with it, and nothing is executed by the webserver.
func dispatch(ctx dbos.Context, name string, payload any) (reads.Execution, error) {
	handle, err := ctx.Enqueue(ctx, queueNameFor(name), name, payload)
	if err != nil {
		return reads.Execution{}, fmt.Errorf("core: enqueue %s: %w", name, err)
	}
	id := handle.GetWorkflowID()
	// The ACK row shows badge + date + id: read the new execution's
	// created_at (the dispatch itself already wrote it to the queue).
	statuses, err := reads.ListByID(ctx, id)
	if err != nil {
		return reads.Execution{}, fmt.Errorf("core: enqueue status %s: %w", id, err)
	}
	if len(statuses) != 1 {
		return reads.Execution{}, fmt.Errorf("core: enqueue status %s: %d rows, want 1", id, len(statuses))
	}
	st, err := reads.MapState(statuses[0].Status)
	if err != nil {
		return reads.Execution{}, err
	}
	return reads.Execution{ID: id, State: st, CreatedAt: statuses[0].CreatedAt}, nil
}
