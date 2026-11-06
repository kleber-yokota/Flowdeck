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

// RegisterTestWorkflows registers the test-only workflows on the env
// context (failing-test · resume-test · live-test · noinput-test ·
// block-test · fork-test · struct-test · linear-test · inner-test ·
// child-test), and the frozen queue (worker concurrency 0 — the
// deterministic queued-state fixture: a run enqueued on it is never
// claimed · it stays ENQUEUED until the owning test cancels it).
func RegisterTestWorkflows(c dbos.Context) {
	dbos.RegisterWorkflow(c, failingTestWorkflow,
		dbos.WithWorkflowName("failing-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, flakyTestWorkflow,
		dbos.WithWorkflowName("resume-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, liveTestWorkflow,
		dbos.WithWorkflowName("live-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, noinputTestWorkflow,
		dbos.WithWorkflowName("noinput-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, blockTestWorkflow,
		dbos.WithWorkflowName("block-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, forkTestWorkflow,
		dbos.WithWorkflowName("fork-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, structTestWorkflow,
		dbos.WithWorkflowName("struct-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, linearTestWorkflow,
		dbos.WithWorkflowName("linear-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, innerTestWorkflow,
		dbos.WithWorkflowName("inner-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)
	dbos.RegisterWorkflow(c, childTestWorkflow,
		dbos.WithWorkflowName("child-test"),
		dbos.WithMaxRecoveryAttempts(3),
	)

	// The frozen queue (worker concurrency 0): the executor never claims
	// a run enqueued on it — the deterministic queued-state fixture.
	dbos.RegisterQueue(c, "frozen", dbos.WithWorkerConcurrency(0))
}

// defaultMsg is the default value of the msg input of the test workflows
// (the parser extracts the const default<NamePascal>, the web form prefills
// it).
const defaultMsg = "FlowDeck"
