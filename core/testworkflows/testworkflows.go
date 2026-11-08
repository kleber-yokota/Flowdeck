// Package testworkflows is the source of the test workflows: the
// registrations the test binaries apply to the env DBOS context (via
// RegisterTestWorkflows from their TestMain) and the parser's fixture —
// the suites point the workflows source at this package directory, and the
// possible-paths graph is built from the registrations and the workflow
// funcs below. Registered here, never part of the workflows package, so the
// fixtures do not appear in the production registry.
package testworkflows

import (
	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// ByName is the registration of the ten test-only workflows keyed by
// name: failing-test · resume-test · live-test · noinput-test ·
// block-test · fork-test · struct-test · linear-test · inner-test ·
// child-test — each entry registers its own workflow with its own
// options (the name and the attempts live in the entries). The map is
// the single source of the entries: Workflows derives from it, and the
// per-test boot registers the names it names (the subset).
var ByName = map[string]func(dbos.Context){
	"failing-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, failingTestWorkflow,
			dbos.WithWorkflowName("failing-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"resume-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, flakyTestWorkflow,
			dbos.WithWorkflowName("resume-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"live-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, liveTestWorkflow,
			dbos.WithWorkflowName("live-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"noinput-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, noinputTestWorkflow,
			dbos.WithWorkflowName("noinput-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"block-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, blockTestWorkflow,
			dbos.WithWorkflowName("block-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"fork-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, forkTestWorkflow,
			dbos.WithWorkflowName("fork-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"struct-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, structTestWorkflow,
			dbos.WithWorkflowName("struct-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"linear-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, linearTestWorkflow,
			dbos.WithWorkflowName("linear-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"inner-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, innerTestWorkflow,
			dbos.WithWorkflowName("inner-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
	"child-test": func(c dbos.Context) {
		dbos.RegisterWorkflow(c, childTestWorkflow,
			dbos.WithWorkflowName("child-test"),
			dbos.WithMaxRecoveryAttempts(3),
		)
	},
}

// Workflows is the registration of the ten test-only workflows — the
// entries of ByName, one per workflow (the name and the attempts live in
// the entries).
var Workflows = func() (list []func(dbos.Context)) {
	for _, entry := range ByName {
		list = append(list, entry)
	}
	return
}()

// Queue is a named queue registration: the queue name and its worker
// concurrency (0 = the executor never claims from it — the deterministic
// queued-state fixture).
type Queue struct {
	Name        string
	Concurrency int
}

// Queues is the named queue registrations: the frozen queue (worker
// concurrency 0 — the deterministic queued-state fixture: a run enqueued
// on it is never claimed · it stays ENQUEUED until the owning test
// cancels it) and its twin live-frozen (the live tests' second queue).
var Queues []Queue = []Queue{
	{Name: "frozen", Concurrency: 0},
	{Name: "live-frozen", Concurrency: 0},
}

// RegisterQueues registers the fixture queues (frozen · live-frozen) on
// the live context — the test-core infrastructure (the per-test boot
// registers them even when it registers a subset of the workflows).
func RegisterQueues(c dbos.Context) {
	for _, q := range Queues {
		dbos.RegisterQueue(c, q.Name, dbos.WithWorkerConcurrency(q.Concurrency))
	}
}

// RegisterTestWorkflows registers the ten test-only workflows and the
// fixture queues on the live context — the lists: each entry registers
// its own workflow or queue (only what the lists name is registered).
func RegisterTestWorkflows(c dbos.Context) {
	for _, f := range Workflows {
		f(c)
	}
	RegisterQueues(c)
}

// defaultMsg is the default value of the msg input of the test workflows
// (the parser extracts the const default<NamePascal>, the web form prefills
// it).
const defaultMsg = "FlowDeck"
