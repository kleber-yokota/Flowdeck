// Package testcore is the shared setup of the core test binaries: the
// per-test boot (BootTest — one explicit boot per test, on a fresh sqlite
// file under the test's TempDir) and the shared poll helpers. Every booted
// test boots its own core and shuts it down (the t.Cleanup): there is no
// env boot and no skip — the default state is not-booted.
package testcore

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testworkflows"
)

// BootTest brings the test core up for one test and shuts it down when
// the test ends: the explicit boot on app flowdeck_test with a fresh
// sqlite file under the test's TempDir, and the Shutdown in a t.Cleanup
// (the TempDir removal is the framework's). The argument is generic:
//   - a string: the workflowsDir (relative to the test binary's
//     directory) — the paths graph from that source tree plus the full
//     registration (the ten test workflows and the fixture queues);
//   - a []string: the names of the test workflows to register (a subset
//     of the ten — an unknown name fails the test) plus the fixture
//     queues (the test-core infrastructure); there is no source dir, so
//     no paths graph.
//
// Tests read the live context from the return — no env, no shared DB.
func BootTest(t *testing.T, dirOrNames any) dbos.Context {
	t.Helper()
	if err := boot.Boot(context.Background(), "flowdeck_test",
		"sqlite:"+path.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("boot: %v", err)
	}
	c := boot.Ctx()
	switch v := dirOrNames.(type) {
	case string:
		if err := boot.SetWorkflowsSource(os.DirFS(v)); err != nil {
			t.Fatalf("set workflows source: %v", err)
		}
		testworkflows.RegisterTestWorkflows(c)
	case []string:
		registerNamed(t, c, v)
	default:
		t.Fatalf("BootTest: unsupported argument %T (want string · the source dir, or []string · the workflow names)", dirOrNames)
	}
	if err := boot.Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}
	t.Cleanup(func() { boot.Shutdown(5 * time.Second) })
	return boot.Ctx()
}

// registerNamed registers the named test workflows (a subset of the ten —
// an unknown name fails the test) plus the fixture queues (the test-core
// infrastructure).
func registerNamed(t *testing.T, c dbos.Context, names []string) {
	t.Helper()
	for _, name := range names {
		entry, ok := testworkflows.ByName[name]
		if !ok {
			t.Fatalf("unknown workflow %q (want a name of the testworkflows fixtures)", name)
		}
		entry(c)
	}
	testworkflows.RegisterQueues(c)
}

// PollState polls the badge state until the wanted one (a success stops
// the poll too — the run finished, just not with the wanted state).
func PollState(t *testing.T, id string, want status.State) (status.State, error) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for {
		st, err := reads.Status(id)
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
func MustStepNames(t *testing.T, ex reads.Execution) []string {
	t.Helper()
	path, err := reads.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	names := make([]string, 0, len(path.Steps))
	for _, s := range path.Steps {
		names = append(names, s.Name)
	}
	return names
}
