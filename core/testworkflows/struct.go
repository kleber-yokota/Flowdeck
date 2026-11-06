package testworkflows

import (
	"context"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// defaultName, defaultStartDate and defaultCount are the defaults of the
// struct-test input (the parser extracts the const default<NamePascal> per
// field — the string and the int form types).
const (
	defaultName      = "FlowDeck"
	defaultStartDate = "2026-01-01"
	defaultCount     = 0
)

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
