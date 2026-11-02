// Package testcore is the shared setup of the core test binaries: the
// env-driven lifecycle (the paths graph from the workflows tree on disk,
// and — when the env context exists — the test workflows and the executor)
// and the shared poll helpers. Every core test binary calls Setup from
// its TestMain; the env-var absence is handled by the NeedCore skip.
package testcore

import (
	"fmt"
	"os"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/status"
)

// Setup brings the env-driven core up for the test binary: the paths
// graph from the workflows tree on disk (workflowsDir, relative to the
// test binary's directory), and — when the env context exists — the test
// workflows (failing-test · resume-test) and the executor. The env-var
// absence is not an error: the NeedCore skip handles it (the mock tests
// run without it).
func Setup(workflowsDir string) {
	if err := boot.SetWorkflowsSource(os.DirFS(workflowsDir)); err != nil {
		fmt.Fprintln(os.Stderr, "testcore: workflows source:", err)
		os.Exit(1)
	}
	c := boot.Ctx()
	if c == nil {
		return
	}
	RegisterTestWorkflows(c)
	if err := boot.Launch(); err != nil {
		fmt.Fprintln(os.Stderr, "testcore: launch:", err)
		os.Exit(1)
	}
}

// NeedCore skips the test when the env-driven context is absent (the env
// vars were not exported before the test binary started).
func NeedCore(t *testing.T) {
	t.Helper()
	if boot.Ctx() == nil {
		t.Skip("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — source .env.test")
	}
}

// PollState polls the badge state until the wanted one (a success stops
// the poll too — the run finished, just not with the wanted state).
func PollState(t *testing.T, id string, want status.State) (status.State, error) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		st, err := core.Status(id)
		if err != nil {
			return st, fmt.Errorf("Status: %w", err)
		}
		if st == want || st == status.Success {
			return st, nil
		}
		if time.Now().After(deadline) {
			return st, fmt.Errorf("poll deadline, last state %s", st)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// MustStepNames returns the step names of the run, failing the test if
// the steps cannot be read.
func MustStepNames(t *testing.T, ex core.Execution) []string {
	t.Helper()
	path, err := core.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	names := make([]string, 0, len(path.Steps))
	for _, s := range path.Steps {
		names = append(names, s.Name)
	}
	return names
}
