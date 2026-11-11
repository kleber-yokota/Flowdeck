package graph

import "testing"

// The read-back dedupes: two branches that lead to the same step sequence
// are one path (duplicates dropped).
func TestPathsDedupeEqualBranchSequences(t *testing.T) {
	body, err := parseSnippet(`
func wf(ctx dbos.Context) error {
	if ok {
		_ = dbos.RunAsStep(ctx, aStep(), dbos.WithStepName("a"))
	} else {
		_ = dbos.RunAsStep(ctx, bStep(), dbos.WithStepName("a"))
	}
	return nil
}
`)
	if err != nil {
		t.Fatalf("parse snippet: %v", err)
	}
	g := buildGraph(body)
	if !equalPaths(g.Paths, [][]string{{"a"}}) {
		t.Errorf("paths: got %v, want [[a]] (the duplicate dropped)", g.Paths)
	}
}
