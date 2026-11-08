package web

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/testworkflows"
)

// bootedTestServer builds the full-stack server of a booted core (no
// stubs): the env-driven core (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL)
// with the test workflows
// registered, this package's TestMain started the executor and built the
// paths graph from the testworkflows source, and New wires every route.
func bootedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	needCore(t)
	return httptest.NewServer(New(core.NewPort(), boot.Ready))
}

// TestMain brings the env-driven core up for the booted tests: the paths
// graph from the testworkflows source (the parser fixture, the unit twin
// of main.go's //go:embed workflows), and — when the env values are set —
// the explicit boot, the test workflows registered on the live context
// and the executor (the unit tests run without them).
func TestMain(m *testing.M) {
	if err := boot.SetWorkflowsSource(os.DirFS("../core/testworkflows")); err != nil {
		fmt.Fprintln(os.Stderr, "web: set workflows source:", err)
		os.Exit(1)
	}
	if appName := os.Getenv("DBOS_APP_NAME"); appName != "" && os.Getenv("DBOS_SYSTEM_DATABASE_URL") != "" {
		if err := boot.Boot(context.Background(), appName, os.Getenv("DBOS_SYSTEM_DATABASE_URL")); err != nil {
			fmt.Fprintln(os.Stderr, "web: boot:", err)
			os.Exit(1)
		}
		testworkflows.RegisterTestWorkflows(boot.Ctx())
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
		t.Skip("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — the env boot is the e2e job's path")
	}
}

// skipBooted skips the test when the env-driven context is up: the test
// exercises the real not-launched port (the 503 and empty-state paths),
// which only exists in the unit mode.
func skipBooted(t *testing.T) {
	t.Helper()
	if boot.Ctx() != nil {
		t.Skip("the env core is booted: the not-launched path is the unit mode (run without the DBOS env)")
	}
}
