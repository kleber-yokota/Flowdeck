// Package core is the DBOS side of FlowDeck — the process-wide singleton
// that owns the DBOS context and the single queue: the lifecycle (the
// env-driven init · Launch · Shutdown · Ready · Ctx), the registry read
// (List), the dispatch (Enqueue), the run reads (Status · Executions ·
// Detail · Steps), the parameter spec (Spec), resume (Resume), cancel
// (Cancel) — and the Port, the single contact the web talks to. The
// workflow packages register their workflows on it via Ctx.
package core

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"sync"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/graph"
)

// ErrNotLaunched is returned by Ready when the core is not booted and
// launched.
var ErrNotLaunched = errors.New("core: not launched")

// coreState is the DBOS component of this package — the process-wide
// singleton: the DBOS context, the boot/launch flags and the path graph
// (built from the workflows package's embedded source, by
// SetWorkflowsSource). Architecturally, the web and the core are two
// separate things (same process/docker): the web queries the core
// (List, Enqueue) and never touches DBOS/Postgres directly.
type coreState struct {
	mu       sync.RWMutex
	dbosCtx  dbos.Context
	booted   bool
	launched bool
	paths    *graph.PathGraph
}

var theCore = &coreState{}

// init brings up the DBOS context from the environment — the production
// path: docker compose sets DBOS_APP_NAME and DBOS_SYSTEM_DATABASE_URL, so
// the context exists before main runs and the workflow files (which import
// this package, and therefore init after it) register their workflows in
// their own init.
// Without env (unit tests, tools) the context stays nil: the explicit
// Boot creates it on demand.
func init() {
	appName := os.Getenv("DBOS_APP_NAME")
	dbURL := os.Getenv("DBOS_SYSTEM_DATABASE_URL")
	if appName == "" || dbURL == "" {
		return
	}
	theCore.mu.Lock()
	defer theCore.mu.Unlock()
	if err := boot(context.Background(), appName, dbURL); err != nil {
		slog.Error("core: dbos context from env", "err", err)
	}
}

// boot creates the DBOS context on the singleton. The caller holds
// theCore.mu.
func boot(ctx context.Context, appName, databaseURL string) error {
	c, err := dbos.NewContext(ctx, dbos.Config{
		AppName:     appName,
		DatabaseURL: databaseURL,
	})
	if err != nil {
		return fmt.Errorf("dbos boot: %w", err)
	}
	theCore.dbosCtx = c
	theCore.booted = true
	theCore.launched = false
	return nil
}

// SetWorkflowsSource builds the possible-paths graph (phase 4, STEP 1):
// the parser (go/parser + go/ast) reads the source of the workflow packages
// (the whole workflows tree, subpackages included — main.go embeds it with
// //go:embed), finds each file's RegisterWorkflow registration and the
// workflow funcs' RunAsStep sequences (if/else forks). Once at boot, never
// per request — the graph is served by plain map lookup. main calls it
// before the web is wired; the tests call it with the workflows tree on
// disk.
func SetWorkflowsSource(fsys fs.FS) error {
	g, err := graph.New(fsys)
	if err != nil {
		return fmt.Errorf("core: build paths: %w", err)
	}
	theCore.mu.Lock()
	theCore.paths = g
	theCore.mu.Unlock()
	return nil
}

// Launch starts the DBOS executor on the singleton context. One executor
// per process: a second Launch without a Shutdown in between returns an
// error.
func Launch() error {
	theCore.mu.Lock()
	defer theCore.mu.Unlock()
	if !theCore.booted {
		return errors.New("core: Launch needs a booted core (env DBOS context or Boot)")
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
	if err := dbos.Launch(theCore.dbosCtx); err != nil {
		return fmt.Errorf("dbos launch: %w", err)
	}
	warnUnregistered(theCore.dbosCtx, theCore.paths)
	theCore.launched = true
	return nil
}

// warnUnregistered compares the possible-paths graph (built from the
// embedded source — no imports involved) with the DBOS registry (built by
// the workflow files' init). A name the source declares but the registry
// lacks means workflows/zz_generated.go is stale — the subpackage's
// registration never ran.
func warnUnregistered(c dbos.Context, paths *graph.PathGraph) {
	if paths == nil {
		return
	}
	registered := map[string]bool{}
	for _, e := range dbos.ListRegisteredWorkflows(c) {
		name := e.Name
		if name == "" {
			name = e.FQN
		}
		registered[name] = true
	}
	for name := range paths.ByName {
		if !registered[name] {
			slog.Warn("workflow declared in source but not registered — run 'go generate ./workflows'", "workflow", name)
		}
	}
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

// Ctx returns the singleton DBOS context — nil until the package init
// (env) or a Boot creates it, and nil after Shutdown. The workflow files
// use it to register their workflows directly in the DBOS (see the
// workflows package).
func Ctx() dbos.Context {
	theCore.mu.RLock()
	defer theCore.mu.RUnlock()
	return theCore.dbosCtx
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
