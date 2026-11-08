package core_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"flowdeck/core/boot"
	"flowdeck/core/testworkflows"
)

// TestMain brings the env-driven core up for the e2e tests: the explicit
// boot from the env vars (the e2e job provides them), the testworkflows
// source, the registration and the executor — no env means no boot (the
// per-test tests boot their own cores).
func TestMain(m *testing.M) {
	if appName := os.Getenv("DBOS_APP_NAME"); appName != "" && os.Getenv("DBOS_SYSTEM_DATABASE_URL") != "" {
		if err := boot.Boot(context.Background(), appName, os.Getenv("DBOS_SYSTEM_DATABASE_URL")); err != nil {
			fmt.Fprintln(os.Stderr, "core_test: boot:", err)
			os.Exit(1)
		}
		if err := boot.SetWorkflowsSource(os.DirFS("testworkflows")); err != nil {
			fmt.Fprintln(os.Stderr, "core_test: set workflows source:", err)
			os.Exit(1)
		}
		testworkflows.RegisterTestWorkflows(boot.Ctx())
		if err := boot.Launch(); err != nil {
			fmt.Fprintln(os.Stderr, "core_test: launch:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}
