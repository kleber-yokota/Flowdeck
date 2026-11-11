package reads_test

import (
	"errors"
	"strings"
	"testing"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
	"flowdeck/core/testworkflows"
)

// A not-launched core returns ErrNotLaunched from every read (the web maps
// it to a 503).
func TestReadsOfUnbootedCore(t *testing.T) {
	if _, err := reads.Status("00000000-0000-0000-0000-000000000000"); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Status unbooted: got %v, want ErrNotLaunched", err)
	}
	if _, err := reads.Detail("00000000-0000-0000-0000-000000000000"); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Detail unbooted: got %v, want ErrNotLaunched", err)
	}
	if _, err := reads.Steps("00000000-0000-0000-0000-000000000000"); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Steps unbooted: got %v, want ErrNotLaunched", err)
	}
	if _, err := reads.StepDetailOf("00000000-0000-0000-0000-000000000000", "s"); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("StepDetail unbooted: got %v, want ErrNotLaunched", err)
	}
	if _, err := reads.Executions("live-test", 10, 0); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Executions unbooted: got %v, want ErrNotLaunched", err)
	}
}

// A broken status read is a real error, not an unknown execution: the
// second connection drops the status table, the live read re-reads the
// schema and fails (the real-DB read-failure state, no mock).
func TestStatusReadFailure(t *testing.T) {
	c, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.DropTable(t, dbfile, "workflow_status")
	if _, err := reads.Status(ex.ID); err == nil {
		t.Fatal("Status after dropped status table: got nil, want a read failure")
	}
	_ = c
}

// A status the SDK cannot produce is rejected, not guessed.
func TestStatusOfUnmappedState(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.SetUnmappedState(t, dbfile, ex.ID)
	if _, err := reads.Status(ex.ID); err == nil || !strings.Contains(err.Error(), "unmapped") {
		t.Fatalf("Status unmapped: got %v, want the unmapped-state rejection", err)
	}
}

func TestDetailReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.DropTable(t, dbfile, "workflow_status")
	if _, err := reads.Detail(ex.ID); err == nil {
		t.Fatal("Detail after dropped status table: got nil, want a read failure")
	}
}

func TestDetailOfUnmappedState(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.SetUnmappedState(t, dbfile, ex.ID)
	if _, err := reads.Detail(ex.ID); err == nil || !strings.Contains(err.Error(), "unmapped") {
		t.Fatalf("Detail unmapped: got %v, want the unmapped-state rejection", err)
	}
}

func TestDetailStepsReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.DropTable(t, dbfile, "operation_outputs")
	if _, err := reads.Detail(ex.ID); err == nil {
		t.Fatal("Detail after dropped events table: got nil, want a read failure")
	}
}

// A child run in a state the SDK cannot produce makes the child read fail:
// the run's path carries the child id, and the child rows' state must map.
func TestDetailChildStateReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("child-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	path, err := reads.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	childID := ""
	for _, s := range path.Steps {
		if s.Kind == reads.KindChild {
			childID = s.ChildID
		}
	}
	if childID == "" {
		t.Fatal("no child row on the child-dispatching run")
	}
	testcore.SetUnmappedState(t, dbfile, childID)
	if _, err := reads.Detail(ex.ID); err == nil {
		t.Fatal("Detail with a BOGUS child state: got nil, want a read failure")
	}
}

// childA and childB are the two children of the double-child run (the
// fixture dir has no run that dispatches two children — the two-child sort
// branch of the child read needs them).
func childAWorkflow(ctx dbos.Context, msg string) (string, error) {
	return msg, nil
}

func childBWorkflow(ctx dbos.Context, msg string) (string, error) {
	return msg, nil
}

func doubleChildWorkflow(ctx dbos.Context, msg string) (string, error) {
	a, err := dbos.RunWorkflow(ctx, childAWorkflow, msg)
	if err != nil {
		return "", err
	}
	ra, err := a.GetResult()
	if err != nil {
		return "", err
	}
	b, err := dbos.RunWorkflow(ctx, childBWorkflow, msg)
	if err != nil {
		return "", err
	}
	rb, err := b.GetResult()
	if err != nil {
		return "", err
	}
	return ra + " " + rb, nil
}

// The children of a run are sorted by id (deterministic display).
func TestDetailChildrenOfATwoChildRunAreSorted(t *testing.T) {
	testcore.BootTestDB(t, "../testworkflows", func(c dbos.Context) {
		testworkflows.RegisterTestWorkflows(c)
		dbos.RegisterWorkflow(c, childAWorkflow, dbos.WithWorkflowName("child-a"))
		dbos.RegisterWorkflow(c, childBWorkflow, dbos.WithWorkflowName("child-b"))
		dbos.RegisterWorkflow(c, doubleChildWorkflow, dbos.WithWorkflowName("double-child-test"))
	})
	ex, err := actions.Enqueue("double-child-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	d, err := reads.Detail(ex.ID)
	if err != nil {
		t.Fatalf("Detail: %v", err)
	}
	if len(d.Children) != 2 {
		t.Fatalf("children: got %v, want two", d.Children)
	}
	if d.Children[0].ID >= d.Children[1].ID {
		t.Fatalf("children not sorted by id: %v", d.Children)
	}
}

func TestExecutionsReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	if _, err := actions.Enqueue("live-test", "FlowDeck"); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.DropTable(t, dbfile, "workflow_status")
	if _, err := reads.Executions("live-test", 10, 0); err == nil {
		t.Fatal("Executions after dropped status table: got nil, want a read failure")
	}
}

// A row in a state the SDK cannot produce makes the listing fail: the
// listing must reject it, not skip or guess it.
func TestExecutionsOfAnUnmappedRow(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("live-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.SetUnmappedState(t, dbfile, ex.ID)
	if _, err := reads.Executions("live-test", 10, 0); err == nil || !strings.Contains(err.Error(), "unmapped") {
		t.Fatalf("Executions unmapped row: got %v, want the unmapped-state rejection", err)
	}
}

// A cancelled run never wrote an output: the listing renders it empty
// (no guessing — the SDK leaves the output unset for a run that did not
// complete).
func TestExecutionsNoOutputRendersEmpty(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("block-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := actions.Cancel(ex.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	page, err := reads.Executions("block-test", 10, 0)
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(page.Rows) != 1 || page.Rows[0].State != status.Cancelled || page.Rows[0].Output != "" {
		t.Fatalf("output: got %+v, want one cancelled row with an empty output", page.Rows)
	}
}

// numWorkflow returns an int: the stored output is the JSON text "42",
// which is not a JSON string — the listing renders it empty (no guessing).
func numWorkflow(ctx dbos.Context, msg int) (int, error) {
	return 42, nil
}

func TestExecutionsNonStringOutputRendersEmpty(t *testing.T) {
	testcore.BootTestDB(t, "../testworkflows", func(c dbos.Context) {
		testworkflows.RegisterTestWorkflows(c)
		dbos.RegisterWorkflow(c, numWorkflow, dbos.WithWorkflowName("num-test"))
	})
	ex, err := actions.Enqueue("num-test", 7)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	page, err := reads.Executions("num-test", 10, 0)
	if err != nil {
		t.Fatalf("Executions: %v", err)
	}
	if len(page.Rows) != 1 || page.Rows[0].Output != "" {
		t.Fatalf("output: got %+v, want one row with an empty output", page.Rows)
	}
}

func TestStepsReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.DropTable(t, dbfile, "workflow_status")
	if _, err := reads.Steps(ex.ID); err == nil {
		t.Fatal("Steps after dropped status table: got nil, want a read failure")
	}
}

func TestStepsOfUnmappedState(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.SetUnmappedState(t, dbfile, ex.ID)
	if _, err := reads.Steps(ex.ID); err == nil || !strings.Contains(err.Error(), "unmapped") {
		t.Fatalf("Steps unmapped: got %v, want the unmapped-state rejection", err)
	}
}

func TestStepsStepRowsReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.DropTable(t, dbfile, "operation_outputs")
	if _, err := reads.Steps(ex.ID); err == nil {
		t.Fatal("Steps after dropped step rows: got nil, want a read failure")
	}
}

func TestStepsChildStateReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("child-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	path, err := reads.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	childID := ""
	for _, s := range path.Steps {
		if s.Kind == reads.KindChild {
			childID = s.ChildID
		}
	}
	if childID == "" {
		t.Fatal("no child row on the child-dispatching run")
	}
	testcore.SetUnmappedState(t, dbfile, childID)
	if _, err := reads.Steps(ex.ID); err == nil {
		t.Fatal("Steps with a BOGUS child state: got nil, want a read failure")
	}
}

func TestStepDetailReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.DropTable(t, dbfile, "workflow_status")
	if _, err := reads.StepDetailOf(ex.ID, "noinput"); err == nil {
		t.Fatal("StepDetail after dropped status table: got nil, want a read failure")
	}
}

func TestStepDetailStepRowsReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("noinput-test", nil)
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	testcore.DropTable(t, dbfile, "operation_outputs")
	if _, err := reads.StepDetailOf(ex.ID, "noinput"); err == nil {
		t.Fatal("StepDetail after dropped step rows: got nil, want a read failure")
	}
}

// The detail read's children: a child row the SDK cannot produce (the
// second connection sets the child's status BOGUS) rejects the read
// (the children fragment cannot render an unknown state).
func TestDetailChildrenReadFailure(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../testworkflows")
	ex, err := actions.Enqueue("child-test", "FlowDeck")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Fatalf("poll to success: %v", err)
	}
	path, err := reads.Steps(ex.ID)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	childID := ""
	for _, s := range path.Steps {
		if s.Kind == reads.KindChild {
			childID = s.ChildID
		}
	}
	if childID == "" {
		t.Fatal("no child row on the child-dispatching run")
	}
	testcore.SetUnmappedState(t, dbfile, childID)
	if _, err := reads.Detail(ex.ID); err == nil || !strings.Contains(err.Error(), "unmapped workflow state") {
		t.Fatalf("Detail with a BOGUS child state: got %v, want the unmapped-state error", err)
	}
}
