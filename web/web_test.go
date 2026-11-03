package web

import (
	"fmt"
	"net/http/httptest"
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"

	// The workflow files register themselves in the env-driven DBOS
	// context (their init) — the booted tests dispatch through the
	// catalog and need the registry populated.
	_ "flowdeck/workflows"
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
