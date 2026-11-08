package catalog

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

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
// package's routes against the live core (the per-test boot —
// bootedTestCore: the fresh sqlite file, RegisterTestWorkflows, and the
// executor the boot launched).
func bootedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	bootedTestCore(t)
	return testServer(t, core.NewPort())
}

// bootedTestCore brings the test core up for one test and shuts it down
// when the test ends: the explicit boot on app flowdeck_test with a fresh
// sqlite file under the test's TempDir, the paths graph from the
// testworkflows source, the test workflows and the fixture queues
// registered on the live context — and the Shutdown in the t.Cleanup (the
// TempDir removal is the framework's). Tests read the live context from
// the return — no env, no shared DB.
func bootedTestCore(t *testing.T) dbos.Context {
	t.Helper()
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+path.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("boot: %v", err)
	}
	c := boot.Ctx()
	if err := boot.SetWorkflowsSource(os.DirFS("../../core/testworkflows")); err != nil {
		t.Fatalf("set workflows source: %v", err)
	}
	testworkflows.RegisterTestWorkflows(c)
	if err := boot.Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}
	t.Cleanup(func() { boot.Shutdown(5 * time.Second) })
	return c
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

// skipBooted skips the test when the env-driven context is up: the test
// exercises the real not-launched port (the 503 and empty-state paths),
// which only exists in the unit mode.
func skipBooted(t *testing.T) {
	t.Helper()
	if boot.Ctx() != nil {
		t.Skip("the env core is booted: the not-launched path is the unit mode (run without the DBOS env)")
	}
}
