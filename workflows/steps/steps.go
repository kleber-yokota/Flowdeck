// Package steps holds the shared step functions of the workflows: the leaf
// of the workflows tree (imported by the workflow subpackages, no workflow
// in it).
package steps

import (
	"context"
	"fmt"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// GreetStep returns a step that greets the given message.
// The step wraps a nondeterministic operation (current time): the result
// is recorded in Postgres, and on recovery the step is not re-executed —
// the workflow resumes from the last completed step.
func GreetStep(msg string) dbos.Step[string] {
	return func(ctx context.Context) (string, error) {
		return fmt.Sprintf("hello, %s (%d)", msg, time.Now().Unix()), nil
	}
}

// EchoStep returns a step that echoes the given label — the plain durable
// step behind the linear (pipeline) and branched (decide) workflows.
func EchoStep(label string) dbos.Step[string] {
	return func(ctx context.Context) (string, error) {
		return label, nil
	}
}
