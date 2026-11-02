package web

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/graph"
)

// okReady is the ready stub: the core is up.
func okReady() error { return nil }

// bootedTestServer builds the full-stack server of a booted core (no
// stubs): the env-driven core (core.init + the workflow files' init, see
// .env.test), this package's TestMain started the executor and built the
// paths graph from the workflows tree, and New wires every route.
func bootedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	needCore(t)
	return httptest.NewServer(New(core.NewPort(), boot.Ready))
}

// TestMain brings the env-driven core up for the booted tests: the paths
// graph from the workflows tree on disk (the unit twin of main.go's
// //go:embed workflows) and the executor (only when the env context exists
// — the mock tests run without it).
func TestMain(m *testing.M) {
	if err := boot.SetWorkflowsSource(os.DirFS("../workflows")); err != nil {
		fmt.Fprintln(os.Stderr, "web: set workflows source:", err)
		os.Exit(1)
	}
	if boot.Ctx() != nil {
		if err := boot.Launch(); err != nil {
			fmt.Fprintln(os.Stderr, "web: launch:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// needCore skips the test when the env-driven context is absent (the env
// vars were not exported before the test binary started).
func needCore(t *testing.T) {
	t.Helper()
	if boot.Ctx() == nil {
		t.Skip("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — source .env.test")
	}
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
