package core

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/graph"
)

// ErrNotLaunched is returned by Ready when the core is not booted and
// launched.
var ErrNotLaunched = errors.New("core: not launched")

// ErrUnknownWorkflow is returned by Enqueue when the name is not in the registry.
var ErrUnknownWorkflow = errors.New("core: workflow not registered")

// QueueName is the single DBOS queue every dispatch (Enqueue) goes through.
const QueueName = "flowdeck"

// Workflow is a workflow registered in the core, exposed to the web.
type Workflow struct {
	Name string
}

// registration is a durable workflow registration: apply records it into
// the DBOS context at Launch, enqueue puts a run of it on the queue.
type registration struct {
	name    string
	apply   func(ctx dbos.Context)
	enqueue func(ctx dbos.Context, payload any) (Execution, error)
}

// coreState is the DBOS component of this package — the process-wide
// singleton: the DBOS context, the boot/launch flags, the durable
// registrations and the path graph (built at Boot from this package's
// source). Architecturally, the web and the core are two separate things
// (same process/docker): the web queries the core (List, Enqueue) and
// never touches DBOS/Postgres directly.
type coreState struct {
	mu              sync.RWMutex
	dbosCtx         dbos.Context
	booted          bool
	launched        bool
	registrations   []registration
	registeredNames map[string]struct{}
	paths           *graph.PathGraph
}

var theCore = &coreState{registeredNames: map[string]struct{}{}}

// The source of this package, embedded so the graph parser can read it at
// Boot — inside the container the source is not on disk.
//
//go:embed *.go
var coreSource embed.FS

// register records a durable workflow registration — called from each
// workflow file's init(). apply is the per-file dbos.RegisterWorkflow call,
// applied to the DBOS context at Launch (the context does not exist yet at
// init time); wf fixes the payload type of the enqueue dispatch.
//
// It panics on a duplicate name: registering the same workflow twice is a
// programming error, and failing here (at the source) beats failing deep in
// dbos.RegisterWorkflow at Launch.
func register[P any, R any](name string, apply func(ctx dbos.Context), wf dbos.Workflow[P, R]) {
	theCore.mu.Lock()
	defer theCore.mu.Unlock()
	if _, dup := theCore.registeredNames[name]; dup {
		panic("core: workflow already registered: " + name)
	}
	theCore.registeredNames[name] = struct{}{}
	var zero P // fallback payload: a dispatch without input runs the zero value
	theCore.registrations = append(theCore.registrations, registration{
		name:  name,
		apply: apply,
		enqueue: func(ctx dbos.Context, payload any) (Execution, error) {
			p, ok := payload.(P)
			if !ok { // nil payload (or a mismatched type) → zero payload
				p = zero
			}
			return dispatch(ctx, name, p)
		},
	})
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

// Boot creates the DBOS context (singleton) and connects to the system
// database (Postgres 18 in production), running the system migrations.
// One context per boot cycle: a second Boot without a Shutdown in between
// returns an error.
func Boot(ctx context.Context, appName, databaseURL string) error {
	theCore.mu.Lock()
	defer theCore.mu.Unlock()
	if theCore.booted {
		return errors.New("core: Boot already called (singleton — one DBOS context per process)")
	}
	// Phase 4, STEP 1: parse this package's own source (go/parser + go/ast)
	// and build the possible-paths graph. Once at Boot, never per request —
	// the graph is served by plain map lookup.
	g, err := graph.New(coreSource)
	if err != nil {
		return fmt.Errorf("core: build paths: %w", err)
	}
	theCore.dbosCtx, err = dbos.NewContext(ctx, dbos.Config{
		AppName:     appName,
		DatabaseURL: databaseURL,
	})
	if err != nil {
		return fmt.Errorf("dbos boot: %w", err)
	}
	theCore.paths = g
	theCore.booted = true
	return nil
}

// Launch applies the registrations declared in the workflow files' init()
// to the singleton context and starts the DBOS executor. One executor per
// boot cycle: a second Launch without a Shutdown in between returns an error.
func Launch() error {
	theCore.mu.Lock()
	defer theCore.mu.Unlock()
	if !theCore.booted {
		return errors.New("core: Boot must be called before Launch")
	}
	if theCore.launched {
		return errors.New("core: Launch already called (singleton — one executor)")
	}
	// The single queue every dispatch goes through. Registering it (before
	// Launch) is what makes this process listen to it: the DBOS executor
	// started below polls the queue and runs the claimed jobs.
	if _, err := dbos.RegisterQueue(theCore.dbosCtx, QueueName); err != nil {
		return fmt.Errorf("core: register queue: %w", err)
	}
	for _, r := range theCore.registrations {
		r.apply(theCore.dbosCtx)
	}
	if err := dbos.Launch(theCore.dbosCtx); err != nil {
		return fmt.Errorf("dbos launch: %w", err)
	}
	theCore.launched = true
	return nil
}

// Shutdown tears down the core: it closes the DBOS context and resets the
// singleton, so a later Boot starts clean. Production exits after this;
// tests use it to re-Boot in isolation.
//
// The context is nilled before dbos.Shutdown runs (and outside the lock),
// so List() during shutdown returns nil instead of blocking on the lock
// or touching a shutting-down context.
func Shutdown(timeout time.Duration) {
	theCore.mu.Lock()
	c := theCore.dbosCtx
	theCore.dbosCtx = nil
	theCore.booted = false
	theCore.launched = false
	theCore.paths = nil
	theCore.mu.Unlock()
	if c == nil {
		return
	}
	dbos.Shutdown(c, timeout)
}

// List returns the workflows registered in the core DBOS — the single
// source of truth for the web listing (no hardcoding).
// It returns nil if the core is not booted (or is shutting down): the web
// then renders its "No workflows registered" empty state.
func List() []Workflow {
	theCore.mu.RLock()
	c := theCore.dbosCtx
	theCore.mu.RUnlock()
	if c == nil {
		return nil
	}
	entries := dbos.ListRegisteredWorkflows(c)
	out := make([]Workflow, 0, len(entries))
	for _, e := range entries {
		name := e.Name
		if name == "" {
			name = e.FQN
		}
		out = append(out, Workflow{Name: name})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Enqueue puts a run of the named workflow onto the DBOS queue and returns
// the new execution (id + state + created_at) — the dispatch ACK: the web
// renders the "queued" row with it, and nothing is executed by the
// webserver.
// A nil payload runs the workflow with its zero payload.
func Enqueue(name string, payload any) (Execution, error) {
	theCore.mu.RLock()
	var reg *registration
	for i := range theCore.registrations {
		if theCore.registrations[i].name == name {
			reg = &theCore.registrations[i]
			break
		}
	}
	theCore.mu.RUnlock()

	c, err := ctxOrErr()
	if err != nil {
		return Execution{}, err
	}
	if reg == nil {
		return Execution{}, fmt.Errorf("%w: %s", ErrUnknownWorkflow, name)
	}
	return reg.enqueue(c, payload)
}

// Ready reports whether the core is booted and launched — the health
// contact (a different contact than Port: it is lifecycle state, not a
// domain conversation). Used by the web's /healthz and the container
// healthcheck.
func Ready() error {
	theCore.mu.RLock()
	defer theCore.mu.RUnlock()
	if !theCore.booted || !theCore.launched {
		return ErrNotLaunched
	}
	return nil
}
