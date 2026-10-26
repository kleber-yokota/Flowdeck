package workflows

import (
	"fmt"
	"os"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/status"
)

// The workflows run through the env-driven core: core.init() created the
// DBOS context from DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL (see
// .env.test), this package's inits registered the workflows, and the
// TestMain starts the executor once — the same lifecycle as production.
func TestMain(m *testing.M) {
	core.SetWorkflowsSource(os.DirFS("."))
	if c := core.Ctx(); c != nil {
		if err := core.Launch(); err != nil {
			fmt.Fprintln(os.Stderr, "workflows_test: launch:", err)
			os.Exit(1)
		}
	}
	os.Exit(m.Run())
}

// needCore skips the test when the env-driven context is absent (the env
// vars were not exported before the test binary started).
func needCore(t *testing.T) {
	t.Helper()
	if core.Ctx() == nil {
		t.Skip("DBOS env not set (DBOS_APP_NAME + DBOS_SYSTEM_DATABASE_URL) — source .env.test")
	}
}

// runToSuccess enqueues the workflow with the raw form values and polls
// until the run is terminal.
func runToSuccess(t *testing.T, name string, values map[string]string) core.Execution {
	t.Helper()
	ex, err := core.Enqueue(name, values)
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

// stepNames returns the executed step names of the run.
func stepNames(t *testing.T, ex core.Execution) []string {
	t.Helper()
	steps, err := core.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	names := make([]string, 0, len(steps))
	for _, s := range steps {
		names = append(names, s.Name)
	}
	return names
}

func assertStepNames(t *testing.T, ex core.Execution, want ...string) {
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
	needCore(t)
	ex := runToSuccess(t, "hello-world", map[string]string{"msg": "FlowDeck"})
	assertStepNames(t, ex, "greet")
}

// The pipeline run takes its single possible path: the four steps in order.
func TestPipelineRun(t *testing.T) {
	needCore(t)
	ex := runToSuccess(t, "pipeline", nil)
	assertStepNames(t, ex, "fetch", "parse", "transform", "store")
}

// The decide run takes the branch its payload chooses: a message goes left,
// the zero payload goes right — the else branch is the one the boot parser
// forks on.
func TestDecideRunBranches(t *testing.T) {
	needCore(t)
	left := runToSuccess(t, "decide", map[string]string{"msg": "left"})
	assertStepNames(t, left, "base", "left")
	right := runToSuccess(t, "decide", map[string]string{"msg": ""})
	assertStepNames(t, right, "base", "right")
}

// The buy subpackage run ends successful with its single step — the
// subpackage is wired by the generated blank import, not by hand.
func TestBuyRun(t *testing.T) {
	needCore(t)
	ex := runToSuccess(t, "buy", map[string]string{"msg": "order-42"})
	assertStepNames(t, ex, "charge")
}
