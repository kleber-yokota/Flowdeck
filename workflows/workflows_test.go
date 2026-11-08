package workflows

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/workflows/buy"
	"flowdeck/workflows/decide"
	"flowdeck/workflows/hello"
	"flowdeck/workflows/pipeline"
)

// bootWorkflows brings the test core up for one test, registering the
// given workflows (a subset of the generated All list — the
// warnUnregistered warning is diagnostic) on the live context, and shuts
// it down when the test ends: the explicit boot on app flowdeck_test with
// a fresh sqlite file under the test's TempDir, the paths graph from this
// package's source (os.DirFS(".")) — and the Shutdown in the t.Cleanup
// (the TempDir removal is the framework's). Tests read the live context
// from the return — no env, no skip.
func bootWorkflows(t *testing.T, register ...func(dbos.Context)) dbos.Context {
	t.Helper()
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+path.Join(t.TempDir(), "test.db")); err != nil {
		t.Fatalf("boot: %v", err)
	}
	c := boot.Ctx()
	for _, r := range register {
		r(c)
	}
	if err := boot.SetWorkflowsSource(os.DirFS(".")); err != nil {
		t.Fatalf("set workflows source: %v", err)
	}
	if err := boot.Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}
	t.Cleanup(func() { boot.Shutdown(5 * time.Second) })
	return c
}

// runToSuccess enqueues the workflow with the clean input (the values the
// web sanitized against the spec) and polls until the run is terminal.
func runToSuccess(t *testing.T, name string, payload any) reads.Execution {
	t.Helper()
	ex, err := actions.Enqueue(name, payload)
	if err != nil {
		t.Fatalf("Enqueue %s: %v", name, err)
	}
	if _, err := pollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll %s to success: %v", name, err)
	}
	return ex
}

// pollState polls the badge state until the wanted one (a success stops the
// poll too — the run finished, just not with the wanted state).
func pollState(t *testing.T, id string, want status.State) (status.State, error) {
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

// stepNames returns the executed step names of the run.
func stepNames(t *testing.T, ex reads.Execution) []string {
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

func assertStepNames(t *testing.T, ex reads.Execution, want ...string) {
	t.Helper()
	got := stepNames(t, ex)
	if len(got) != len(want) {
		t.Fatalf("steps of %s: got %v, want %v", ex.ID, got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Fatalf("step %d of %s: got %q, want %q", i, ex.ID, got[i], name)
		}
	}
}

// The hello-world run ends successful with its single step.
func TestHelloWorldRun(t *testing.T) {
	bootWorkflows(t, hello.Register)
	ex := runToSuccess(t, "hello-world", "FlowDeck")
	assertStepNames(t, ex, "greet")
}

// The pipeline run takes its single possible path: the four steps in order.
func TestPipelineRun(t *testing.T) {
	bootWorkflows(t, pipeline.Register)
	ex := runToSuccess(t, "pipeline", nil)
	assertStepNames(t, ex, "fetch", "parse", "transform", "store")
}

// The decide run takes the branch its payload chooses: a message goes left,
// the zero payload goes right — the else branch is the one the boot parser
// forks on.
func TestDecideRunBranches(t *testing.T) {
	bootWorkflows(t, decide.Register)
	left := runToSuccess(t, "decide", "left")
	assertStepNames(t, left, "base", "left")
	right := runToSuccess(t, "decide", "")
	assertStepNames(t, right, "base", "right")
}

// The buy subpackage run ends successful with its single step — the
// subpackage is wired by the generated blank import, not by hand.
func TestBuyRun(t *testing.T) {
	bootWorkflows(t, buy.Register)
	ex := runToSuccess(t, "buy", "order-42")
	assertStepNames(t, ex, "charge")
}
