//go:build e2e

package core

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// Integration test: runs the example workflow (with step) against a real
// Postgres, using the core singleton. Run with:
//
//	DBOS_E2E_DATABASE_URL=postgres://user:pass@host:5432/db \
//	  go test -tags e2e -run TestE2E ./...
func TestE2EHelloWorldRunsStep(t *testing.T) {
	dbURL := os.Getenv("DBOS_E2E_DATABASE_URL")
	if dbURL == "" {
		t.Skip("DBOS_E2E_DATABASE_URL not set — skipping e2e test")
	}

	Shutdown(5 * time.Second) // reset the singleton (nil-safe)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := Boot(ctx, "flowdeck_e2e", dbURL); err != nil {
		t.Fatalf("boot: %v", err)
	}
	defer Shutdown(5 * time.Second)

	if err := Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}

	names := map[string]bool{}
	for _, w := range List() {
		names[w.Name] = true
	}
	if !names["hello-world"] {
		t.Fatalf("registry missing hello-world: %v", names)
	}

	handle, err := dbos.RunWorkflow(dbosCtx, HelloWorld, "FlowDeck")
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
