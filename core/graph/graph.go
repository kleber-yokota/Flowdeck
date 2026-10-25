// Package graph is the possible-paths graph of the registered workflows:
// workflow name → every sequence of steps that can run. It is computed
// once at Boot from the core package's source (phase 4, STEP 1) and served
// by plain map lookup — no re-parse, no DB. It is independent of the DBOS
// core: no Postgres, no executor state.
package graph

import (
	"fmt"
	"io/fs"
	"strings"
)

// PathGraph is the possible-paths graph: workflow name → paths.
type PathGraph struct {
	// ByName is the built graph: workflow name → paths. Read-only after
	// the build — the web holds the graph through the Port and only
	// reads it via Of.
	ByName map[string][][]string
}

// New builds the graph from the core package's source files (test files
// excluded) — the embed.FS of the package that owns the registrations.
func New(fsys fs.FS) (*PathGraph, error) {
	files := map[string][]byte{}
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return nil, fmt.Errorf("list core source: %w", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", name, err)
		}
		files[name] = data
	}
	byName, err := pathsFromSources(files)
	if err != nil {
		return nil, err
	}
	return &PathGraph{ByName: byName}, nil
}

// Of returns the possible paths of the named workflow, or nil when the
// name is unknown.
func (g *PathGraph) Of(name string) [][]string { return g.ByName[name] }
