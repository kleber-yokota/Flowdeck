// Package testworkflows is the source of the test workflows: the
// registrations the test binaries apply to the env DBOS context (via
// RegisterTestWorkflows from their TestMain) and the parser's fixture —
// the suites point the workflows source at this package directory, and the
// possible-paths graph is built from the registrations and the workflow
// funcs below. Registered here, never part of the workflows package, so the
// fixtures do not appear in the production registry.
package testworkflows

import (
	"context"
	"errors"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// RegisterTestWorkflows registers the test-only workflows on the env
// context (failing-test · resume-test · live-test · noinput-test ·
// block-test · fork-test · struct-test · linear-test · inner-test ·
// child-test).
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
}

// FlakyFail makes the flaky test workflow fail in its step — the test-only
// resume setup: a failed run with a known stored input.
var FlakyFail bool

// defaultMsg is the default value of the msg input of the test workflows
// (the parser extracts the const default<NamePascal>, the web form prefills
// it).
const defaultMsg = "FlowDeck"

// defaultName, defaultStartDate and defaultCount are the defaults of the
// struct-test input (the parser extracts the const default<NamePascal> per
// field — the string and the int form types).
const (
	defaultName      = "FlowDeck"
	defaultStartDate = "2026-01-01"
	defaultCount     = 0
)

// failingTestWorkflow is a workflow that fails in a named step — it
// exercises the failure path (retries, Detail: error + the failed step).
func failingTestWorkflow(ctx dbos.Context, _ string) (string, error) {
	_, err := dbos.RunAsStep(ctx, failingTestStep(), dbos.WithStepName("boom-step"))
	if err != nil {
		return "", err
	}
	return "", nil
}

// failingTestStep is a step that always fails.
func failingTestStep() dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return "", errors.New("boom: step failed")
	}
}

// flakyTestWorkflow fails in the "flaky" step while FlakyFail is set, and
// returns msg otherwise.
func flakyTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	return dbos.RunAsStep(ctx, flakyStep(msg), dbos.WithStepName("flaky"))
}

// flakyStep is the step that fails while FlakyFail is set.
func flakyStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		if FlakyFail {
			return "", errors.New("flaky: step failed")
		}
		return msg, nil
	}
}

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

// noinputTestStep is the no-input step: a plain named function (there is
// nothing to capture) — the dispatch passes nil, the parser skips the blank
// parameter.
func noinputTestStep(_ context.Context) (string, error) {
	return "ok", nil
}

// noinputTestWorkflow is the no-input workflow: no form, one Run button,
// the run reaches success fast (the dispatch and the resume tests).
func noinputTestWorkflow(ctx dbos.Context, _ string) (string, error) {
	return dbos.RunAsStep(ctx, noinputTestStep, dbos.WithStepName("noinput"))
}

// forkStep is the echo step of the fork-test workflow (the branch is
// chosen by the payload at the workflow level, not in the step).
func forkStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return msg, nil
	}
}

// forkTestWorkflow is the branch fixture: after the base step, the second
// step depends on the payload — two possible paths (base → left and base →
// right): the if the parser forks on.
func forkTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	base, err := dbos.RunAsStep(ctx, forkStep("base"), dbos.WithStepName("base"))
	if err != nil {
		return "", err
	}
	var choice string
	if msg != "" {
		left, err := dbos.RunAsStep(ctx, forkStep("left"), dbos.WithStepName("left"))
		if err != nil {
			return "", err
		}
		choice = left
	} else {
		right, err := dbos.RunAsStep(ctx, forkStep("right"), dbos.WithStepName("right"))
		if err != nil {
			return "", err
		}
		choice = right
	}
	return base + " " + choice, nil
}

// structTestArgs is the struct-test input: one form item per field (from
// the spec the core parses at boot) — the string and the int form types.
type structTestArgs struct {
	Name      string
	StartDate string
	Count     int
}

// structStep is the echo step of the struct-test workflow.
func structStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return msg, nil
	}
}

// structTestWorkflow is the struct-input fixture: one form item per field,
// prefilled with the defaults of the source (the const default<NamePascal>).
func structTestWorkflow(ctx dbos.Context, args structTestArgs) (string, error) {
	return dbos.RunAsStep(ctx, structStep(args.Name+" @ "+args.StartDate),
		dbos.WithStepName("schedule"))
}

// linearStep is the echo step of the linear-test workflow.
func linearStep(msg string) dbos.Step[string] {
	return func(_ context.Context) (string, error) {
		return msg, nil
	}
}

// linearTestWorkflow is the linear fixture: four steps in a row — a single
// possible path (fetch → parse → transform → store).
func linearTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	fetch, err := dbos.RunAsStep(ctx, linearStep("fetch"), dbos.WithStepName("fetch"))
	if err != nil {
		return "", err
	}
	parse, err := dbos.RunAsStep(ctx, linearStep("parse"), dbos.WithStepName("parse"))
	if err != nil {
		return "", err
	}
	transform, err := dbos.RunAsStep(ctx, linearStep("transform"), dbos.WithStepName("transform"))
	if err != nil {
		return "", err
	}
	store, err := dbos.RunAsStep(ctx, linearStep("store"), dbos.WithStepName("store"))
	if err != nil {
		return "", err
	}
	return fetch + " " + parse + " " + transform + " " + store, nil
}

// innerTestWorkflow is the child fixture: a single-step workflow that
// child-test runs as a child run (the Detail children seam).
func innerTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	return dbos.RunAsStep(ctx, linearStep(msg), dbos.WithStepName("inner"))
}

// childTestWorkflow is the child-dispatch fixture: a base step and a child
// run (the Detail children seam — the child appears in the steps of the
// parent run).
func childTestWorkflow(ctx dbos.Context, msg string) (string, error) {
	base, err := dbos.RunAsStep(ctx, linearStep("base"), dbos.WithStepName("base"))
	if err != nil {
		return "", err
	}
	handle, err := dbos.RunWorkflow(ctx, innerTestWorkflow, msg)
	if err != nil {
		return "", err
	}
	child, err := handle.GetResult()
	if err != nil {
		return "", err
	}
	return base + " " + child, nil
}
