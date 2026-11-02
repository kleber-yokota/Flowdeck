package core

import (
	"fmt"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// QueueName is the single DBOS queue every dispatch (Enqueue) goes through.
const QueueName = "flowdeck"

// Enqueue puts a run of the named workflow onto the DBOS queue and returns
// the new execution (id + state + created_at) — the dispatch ACK: the web
// renders the "queued" row with it, and nothing is executed by the
// webserver.
// The payload is the clean input (the web sanitized it against the
// workflow's spec before the call — the core trusts the caller); a nil
// payload is the zero payload (a dispatch without a form — the SDK decodes
// the stored "null" back into the zero value at run time).
func Enqueue(name string, payload any) (Execution, error) {
	c, err := ctxOrErr()
	if err != nil {
		return Execution{}, err
	}
	for _, w := range List() {
		if w.Name == name {
			return dispatch(c, name, payload)
		}
	}
	return Execution{}, fmt.Errorf("%w: %s", ErrUnknownWorkflow, name)
}

// dispatch puts payload on the queue under name and returns the new
// execution (id + state + created_at) — the dispatch ACK: the web renders
// the "queued" row with it, and nothing is executed by the webserver.
func dispatch(ctx dbos.Context, name string, payload any) (Execution, error) {
	handle, err := ctx.Enqueue(ctx, QueueName, name, payload)
	if err != nil {
		return Execution{}, fmt.Errorf("core: enqueue %s: %w", name, err)
	}
	id := handle.GetWorkflowID()
	// The ACK row shows badge + date + id: read the new execution's
	// created_at (the dispatch itself already wrote it to the queue).
	statuses, err := listByID(ctx, id)
	if err != nil {
		return Execution{}, fmt.Errorf("core: enqueue status %s: %w", id, err)
	}
	if len(statuses) != 1 {
		return Execution{}, fmt.Errorf("core: enqueue status %s: %d rows, want 1", id, len(statuses))
	}
	st, err := mapState(statuses[0].Status)
	if err != nil {
		return Execution{}, err
	}
	return Execution{ID: id, State: st, CreatedAt: statuses[0].CreatedAt}, nil
}
