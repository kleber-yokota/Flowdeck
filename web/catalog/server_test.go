package catalog

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"flowdeck/core"
	"flowdeck/core/graph"
)

// testServer builds the httptest server from the given Port: this
// package's routes on the mux (no middleware — the middlewares are the
// web package's and are tested there).
func testServer(t *testing.T, p core.Port) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux, p)
	return httptest.NewServer(mux)
}

// fakeGraph builds a possible-paths graph holding the given paths under
// name (the stub for the graph component in the unit tests).
func fakeGraph(name string, paths [][]string) *graph.PathGraph {
	root := &graph.Block{ID: "b0", Level: 0, Type: graph.BlockSequence}
	var children []graph.Child
	var steps []string
	seen := map[string]bool{}
	for _, p := range paths {
		for _, s := range p {
			if !seen[s] {
				seen[s] = true
				children = append(children, graph.Child{Node: &graph.Node{Name: s, Kind: graph.NodeStep}})
				steps = append(steps, s)
			}
		}
	}
	root.Children = children
	byName := map[string]*graph.WorkflowGraph{}
	if len(children) > 0 {
		byName[name] = &graph.WorkflowGraph{Root: root, Steps: steps}
	}
	return &graph.PathGraph{ByName: byName}
}
