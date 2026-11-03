package graph

import (
	"testing"
)

// Sequential RunAsStep calls form a single linear path: all the steps,
// in source order.
func TestPossiblePathsLinearSteps(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("one"))
	_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("two"))
	_ = dbos.RunAsStep(ctx, cStep(), dbos.WithStepName("three"))
	_ = dbos.RunAsStep(ctx, dStep(), dbos.WithStepName("four"))
	_ = dbos.RunAsStep(ctx, eStep(), dbos.WithStepName("five"))
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	got := possiblePaths(body)
	if !equalPaths(got, [][]string{{"one", "two", "three", "four", "five"}}) {
		t.Fatalf("possiblePaths: got %v", got)
	}
}

// An if forks the possible paths: the taken branch (if-body steps) and the
// not-taken one (else-body steps).
func TestPossiblePathsBranchOnIf(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	if ok {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("b"))
	} else {
		_ = dbos.RunAsStep(ctx, cStep(), dbos.WithStepName("c"))
	}
	_ = dbos.RunAsStep(ctx, dStep(), dbos.WithStepName("d"))
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	got := possiblePaths(body)
	if !equalPaths(got, [][]string{{"a", "b", "d"}, {"a", "c", "d"}}) {
		t.Fatalf("possiblePaths: got %v", got)
	}
}

// An if without else also leaves the path without the if steps (the branch
// may not be taken).
func TestPossiblePathsBranchWithoutElse(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	if ok {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("b"))
	}
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	if body == nil {
		t.Fatal("snippet: wf func not found")
	}

	got := possiblePaths(body)
	if !equalPaths(got, [][]string{{"a", "b"}, {"a"}}) {
		t.Fatalf("possiblePaths: got %v", got)
	}
}
