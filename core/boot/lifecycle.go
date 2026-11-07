// Package boot is the DBOS root of the core: the process-wide singleton
// that owns the DBOS context and the single queue — the explicit Boot
// (Boot(ctx, appName, databaseURL), the error returned to the caller),
// Launch (the executor, the queue registration, the registry
// verification), Shutdown, the readiness contact (Ready) and Ctx (the
// contact the registration calls). No env read here — the call sites
// (main.go, the test bootstraps) read the env and pass the values. The
// paths graph (SetWorkflowsSource) is the boot map the whole core reads
// from it.
package boot

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sync"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/graph"
)

// ErrNotLaunched is returned by Ready when the core is not booted and
// launched.
var ErrNotLaunched = errors.New("core: not launched")

// QueueName is the single DBOS queue: Launch registers it, and every
// dispatch (Enqueue) and resume (Resume) goes through it.
const QueueName = "flowdeck"

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

// Boot creates the DBOS context on the singleton — the single explicit
// boot: the env values are read by the caller (main.go, the test
// bootstraps) and passed here, and the error is returned to the caller,
// not logged (a failed boot is visible before any launch).
func Boot(ctx context.Context, appName, databaseURL string) error {
	c, err := dbos.NewContext(ctx, dbos.Config{
		AppName:     appName,
		DatabaseURL: databaseURL,
	})
	if err != nil {
		return fmt.Errorf("dbos boot: %w", err)
	}
	theCore.mu.Lock()
	defer theCore.mu.Unlock()
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
		return errors.New("core: Launch needs a booted core (Boot)")
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

// Ctx returns the singleton DBOS context — nil until a Boot creates it,
// and nil after Shutdown. The workflow registration calls
// (workflows.RegisterAll) use it to register their workflows on the
// live context.
func Ctx() dbos.Context {
	theCore.mu.RLock()
	defer theCore.mu.RUnlock()
	return theCore.dbosCtx
}

// CtxOrErr returns the live DBOS context, or ErrNotLaunched — the read
// guard of every query in the core (the core must be booted and launched).
func CtxOrErr() (dbos.Context, error) {
	theCore.mu.RLock()
	defer theCore.mu.RUnlock()
	if !theCore.booted || !theCore.launched || theCore.dbosCtx == nil {
		return nil, ErrNotLaunched
	}
	return theCore.dbosCtx, nil
}

// Paths returns the possible-paths graph (nil before SetWorkflowsSource,
// and nil after Shutdown). The graph is immutable once built — the
// readers map-lookup it directly.
func Paths() *graph.PathGraph {
	theCore.mu.RLock()
	defer theCore.mu.RUnlock()
	return theCore.paths
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

// warnUnregistered compares the possible-paths graph (built from the
// embedded source — no imports involved) with the DBOS registry (built by
// the workflow files' Register calls, via the generated RegisterAll). A
// name the source declares but the registry lacks means
// workflows/zz_generated.go is stale — the subpackage's registration never
// ran.
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
