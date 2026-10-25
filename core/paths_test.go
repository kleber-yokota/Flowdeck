package core

import (
	"slices"
	"testing"
	"time"
)

// Phase 4, STEP 1: Boot parses this package's own source and builds the
// map name → possible paths. hello-world has one RunAsStep ("greet"), so
// its only possible path is [greet].
func TestBootBuildsPathsMap(t *testing.T) {
	bootSQLiteCoreForTest(t)

	got := theCore.paths.Of("hello-world")
	if !equalPaths(got, [][]string{{"greet"}}) {
		t.Fatalf("Paths(hello-world): got %v, want %v", got, [][]string{{"greet"}})
	}
}

// Phase 4, STEP 1: the boot map of the branched workflow (decide) forks on
// the if: base → left and base → right.
func TestBootBuildsPathsMapDecide(t *testing.T) {
	bootSQLiteCoreForTest(t)

	got := theCore.paths.Of("decide")
	if !equalPaths(got, [][]string{{"base", "left"}, {"base", "right"}}) {
		t.Fatalf("Paths(decide): got %v", got)
	}
}

// Phase 4, STEP 1: the boot map of the linear workflow (pipeline) is a
// single path with all the steps in source order.
func TestBootBuildsPathsMapPipeline(t *testing.T) {
	bootSQLiteCoreForTest(t)

	got := theCore.paths.Of("pipeline")
	if !equalPaths(got, [][]string{{"fetch", "parse", "transform", "store"}}) {
		t.Fatalf("Paths(pipeline): got %v", got)
	}
}

// The port before Boot carries the empty graph: Of returns nil (core not
// booted), like List().
func TestPortBeforeBootCarriesEmptyGraph(t *testing.T) {
	Shutdown(5 * time.Second)

	if got := NewPort().Graph.Of("hello-world"); got != nil {
		t.Fatalf("graph before Boot: got %v, want nil", got)
	}
}

func equalPaths(a, b [][]string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !slices.Equal(a[i], b[i]) {
			return false
		}
	}
	return true
}
