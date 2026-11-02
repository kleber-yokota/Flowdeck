package executions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
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

// TestMain brings the env-driven core up for the booted tests: the paths
// graph from the workflows tree on disk, and the executor when the env
// context exists (the mock tests run without it).
func TestMain(m *testing.M) {
	if err := boot.SetWorkflowsSource(os.DirFS("../../workflows")); err != nil {
		fmt.Fprintln(os.Stderr, "executions: set workflows source:", err)
		os.Exit(1)
	}
	if boot.Ctx() != nil {
		if err := boot.Launch(); err != nil {
			fmt.Fprintln(os.Stderr, "executions: launch:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// bootedTestServer builds the server of a booted core (no stubs): this
// package's routes against the live core (the env-driven context, the
// workflow files' init, and the executor this package's TestMain started).
func bootedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	if boot.Ctx() == nil {
		t.Skip("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — source .env.test")
	}
	return testServer(t, core.NewPort())
}
