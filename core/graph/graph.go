// Package graph is the possible-paths graph of the registered workflows:
// workflow name → every sequence of steps that can run. It is computed
// once at boot from the workflows package's embedded source (phase 4,
// STEP 1) and served by plain map lookup — no re-parse, no DB. It is
// independent of the DBOS core: no Postgres, no executor state.
package graph

import (
	"fmt"
	"io/fs"
	"strings"
)

// PathGraph is the possible-paths graph: workflow name → paths, plus the
// parameter spec of each workflow (phase 5, BOOT).
type PathGraph struct {
	// ByName is the built graph: workflow name → paths. Read-only after
	// the build — the web holds the graph through the Port and only
	// reads it via Of.
	ByName map[string][][]string
	// Specs is the built spec map: workflow name → parameter spec
	// (input + step defaults). Read-only after the build.
	Specs map[string]Spec
}

// New builds the graph from the embedded source of the workflow packages —
// it walks the whole FS tree (subpackages included) and parses every .go
// file it finds (test files excluded).
func New(fsys fs.FS) (*PathGraph, error) {
	files := map[string][]byte{}
	walkErr := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			return nil
		}
		data, readErr := fs.ReadFile(fsys, path)
		if readErr != nil {
			return fmt.Errorf("read %s: %w", path, readErr)
		}
		files[path] = data
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	byName, specs, err := pathsFromSources(files)
	if err != nil {
		return nil, err
	}
	return &PathGraph{ByName: byName, Specs: specs}, nil
}

// Of returns the possible paths of the named workflow, or nil when the
// name is unknown.
func (g *PathGraph) Of(name string) [][]string { return g.ByName[name] }
