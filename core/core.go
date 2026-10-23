package core

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// Workflow is a workflow registered in the core, exposed to the web.
type Workflow struct {
	Name string
}

// The DBOS core as a process-wide singleton. Architecturally, the web and
// the core are two separate things (same process/docker): the web queries
// the core (List) and never touches DBOS/Postgres directly.

var (
	mu              sync.RWMutex
	dbosCtx         dbos.Context
	booted          bool
	launched        bool
	registrations   []func(dbos.Context)
	registeredNames = map[string]struct{}{}
)

// register records a durable workflow registration — called from each
// workflow file's init(). It is applied to the DBOS context at Launch
// (the context does not exist yet at init time).
//
// It panics on a duplicate name: registering the same workflow twice is a
// programming error, and failing here (at the source) beats failing deep in
// dbos.RegisterWorkflow at Launch.
func register[P any, R any](name string, wf dbos.Workflow[P, R]) {
	mu.Lock()
	defer mu.Unlock()
	if _, dup := registeredNames[name]; dup {
		panic("core: workflow already registered: " + name)
	}
	registeredNames[name] = struct{}{}
	registrations = append(registrations, func(ctx dbos.Context) {
		dbos.RegisterWorkflow(ctx, wf,
			dbos.WithWorkflowName(name),
			dbos.WithMaxRecoveryAttempts(3),
		)
	})
}

// Boot creates the DBOS context (singleton) and connects to the system
// database (Postgres 18 in production), running the system migrations.
// One context per boot cycle: a second Boot without a Shutdown in between
// returns an error.
func Boot(ctx context.Context, appName, databaseURL string) error {
	mu.Lock()
	defer mu.Unlock()
	if booted {
		return errors.New("core: Boot already called (singleton — one DBOS context per process)")
	}
	var err error
	dbosCtx, err = dbos.NewContext(ctx, dbos.Config{
		AppName:     appName,
		DatabaseURL: databaseURL,
	})
	if err != nil {
		return fmt.Errorf("dbos boot: %w", err)
	}
	booted = true
	return nil
}

// Launch applies the registrations declared in the workflow files' init()
// to the singleton context and starts the DBOS executor. One executor per
// boot cycle: a second Launch without a Shutdown in between returns an error.
func Launch() error {
	mu.Lock()
	defer mu.Unlock()
	if !booted {
		return errors.New("core: Boot must be called before Launch")
	}
	if launched {
		return errors.New("core: Launch already called (singleton — one executor)")
	}
	for _, r := range registrations {
		r(dbosCtx)
	}
	if err := dbos.Launch(dbosCtx); err != nil {
		return fmt.Errorf("dbos launch: %w", err)
	}
	launched = true
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
	mu.Lock()
	c := dbosCtx
	dbosCtx = nil
	booted = false
	launched = false
	mu.Unlock()
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
	mu.RLock()
	c := dbosCtx
	mu.RUnlock()
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
