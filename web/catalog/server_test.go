package catalog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/testworkflows"
)

// testServer builds the httptest server from the given Port: this
// package's routes on the mux (no middleware — the middlewares are the
// web package's and are tested there). The tests use the real port
// (core.NewPort): booted against the env core, or not launched (the 503
// and empty-state paths).
func testServer(t *testing.T, p core.Port) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux, p)
	srv := httptest.NewServer(mux)
	// The close is the first cleanup registered on t: it runs last (LIFO)
	// — after every settle cleanup the test registered (the cancel · the
	// ok dispatch), so the suite converges even when the test fails.
	t.Cleanup(srv.Close)
	return srv
}

// bootedTestServer builds the server of a booted core (no stubs): this
// package's routes against the live core (the env-driven context,
// RegisterTestWorkflows, and the executor this package's TestMain started).
func bootedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	needCore(t)
	return testServer(t, core.NewPort())
}

// TestMain brings the env-driven core up for the booted tests: the paths
// graph from the testworkflows source (the parser fixture), and — when
// the env values are set — the explicit boot, the test workflows
// registered on the live context and the executor (the unit tests run
// without them).
func TestMain(m *testing.M) {
	if err := boot.SetWorkflowsSource(os.DirFS("../../core/testworkflows")); err != nil {
		fmt.Fprintln(os.Stderr, "catalog: set workflows source:", err)
		os.Exit(1)
	}
	if appName := os.Getenv("DBOS_APP_NAME"); appName != "" && os.Getenv("DBOS_SYSTEM_DATABASE_URL") != "" {
		if err := boot.Boot(context.Background(), appName, os.Getenv("DBOS_SYSTEM_DATABASE_URL")); err != nil {
			fmt.Fprintln(os.Stderr, "catalog: boot:", err)
			os.Exit(1)
		}
		testworkflows.RegisterTestWorkflows(boot.Ctx())
		if err := boot.Launch(); err != nil {
			fmt.Fprintln(os.Stderr, "catalog: launch:", err)
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

// skipBooted skips the test when the env-driven context is up: the test
// exercises the real not-launched port (the 503 and empty-state paths),
// which only exists in the unit mode.
func skipBooted(t *testing.T) {
	t.Helper()
	if boot.Ctx() != nil {
		t.Skip("the env core is booted: the not-launched path is the unit mode (run without the DBOS env)")
	}
}
