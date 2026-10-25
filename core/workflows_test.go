package core

import (
	"strings"
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// Test for workflows.go: init() registers the workflow in the core and
// the core runs the workflow with its step. Uses SQLite (temp dir) — no Postgres.
func TestHelloWorldRegistersAndRuns(t *testing.T) {
	bootSQLiteCoreForTest(t)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	// workflows.go init() must have registered "hello-world" in the registry
	names := map[string]bool{}
	for _, w := range List() {
		names[w.Name] = true
	}
	if !names["hello-world"] {
		t.Fatalf("registry missing hello-world: %v", names)
	}

	handle, err := dbos.RunWorkflow(theCore.dbosCtx, HelloWorld, "FlowDeck")
	if err != nil {
		t.Fatalf("RunWorkflow: %v", err)
	}
	result, err := handle.GetResult()
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if !strings.HasPrefix(result, "hello, FlowDeck") {
		t.Fatalf("step result: got %q", result)
	}
}
