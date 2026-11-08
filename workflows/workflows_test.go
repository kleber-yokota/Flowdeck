package workflows

import (
	"reflect"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
	"flowdeck/workflows/buy"
	"flowdeck/workflows/decide"
	"flowdeck/workflows/hello"
	"flowdeck/workflows/pipeline"
)

// runToSuccess enqueues the workflow with the clean input (the values the
// web sanitized against the spec) and polls to success on the shared poll
// (testcore.PollState).
func runToSuccess(t *testing.T, name string, payload any) reads.Execution {
	t.Helper()
	ex, err := actions.Enqueue(name, payload)
	if err != nil {
		t.Fatalf("Enqueue %s: %v", name, err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll %s to success: %v", name, err)
	}
	return ex
}

// The hello-world run ends successful with its single step.
func TestHelloWorldRun(t *testing.T) {
	testcore.BootTest(t, ".", hello.Register)
	ex := runToSuccess(t, "hello-world", "FlowDeck")
	got := testcore.MustStepNames(t, ex)
	want := []string{"greet"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps: got %v, want %v", got, want)
	}
}

// The pipeline run takes its single possible path: the four steps in order.
func TestPipelineRun(t *testing.T) {
	testcore.BootTest(t, ".", pipeline.Register)
	ex := runToSuccess(t, "pipeline", nil)
	got := testcore.MustStepNames(t, ex)
	want := []string{"fetch", "parse", "transform", "store"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps: got %v, want %v", got, want)
	}
}

// The decide run takes the branch its payload chooses: a message goes left,
// the zero payload goes right — the else branch is the one the boot parser
// forks on.
func TestDecideRunBranches(t *testing.T) {
	testcore.BootTest(t, ".", decide.Register)
	left := runToSuccess(t, "decide", "left")
	leftSteps := testcore.MustStepNames(t, left)
	wantLeft := []string{"base", "left"}
	if !reflect.DeepEqual(leftSteps, wantLeft) {
		t.Fatalf("left steps: got %v, want %v", leftSteps, wantLeft)
	}
	right := runToSuccess(t, "decide", "")
	rightSteps := testcore.MustStepNames(t, right)
	wantRight := []string{"base", "right"}
	if !reflect.DeepEqual(rightSteps, wantRight) {
		t.Fatalf("right steps: got %v, want %v", rightSteps, wantRight)
	}
}

// The buy subpackage run ends successful with its single step — the
// subpackage is wired by the generated blank import, not by hand.
func TestBuyRun(t *testing.T) {
	testcore.BootTest(t, ".", buy.Register)
	ex := runToSuccess(t, "buy", "order-42")
	got := testcore.MustStepNames(t, ex)
	want := []string{"charge"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("steps: got %v, want %v", got, want)
	}
}
