package reads_test

import (
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
)

// Executions lists the runs of a workflow, newest first: the run the
// test enqueued is the one row on the fresh per-test file, with its
// final state.
func TestExecutionsListsRunsOfWorkflow(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ex, err := actions.Enqueue("live-test", "one")
	if err != nil {
		t.Errorf("Enqueue: %v", err)
		return
	}
	if _, err := testcore.PollState(t, ex.ID, status.Success); err != nil {
		t.Errorf("poll to success: %v", err)
		return
	}
	page, err := reads.Executions("live-test", 6, 0)
	if err != nil {
		t.Errorf("Executions: %v", err)
		return
	}
	if len(page.Rows) != 1 {
		t.Errorf("Executions(live-test): got %d runs, want exactly the one enqueued", len(page.Rows))
		return
	}
	if page.Rows[0].ID != ex.ID {
		t.Errorf("Executions: got run %s, want %s", page.Rows[0].ID, ex.ID)
	}
	if page.Rows[0].State != status.Success {
		t.Errorf("execution %s state: got %s, want success", page.Rows[0].ID, page.Rows[0].State)
	}
}

// Executions of a name with no runs is empty (no error).
func TestExecutionsUnknownWorkflowIsEmpty(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	page, err := reads.Executions("no-such-workflow", 6, 0)
	if err != nil {
		t.Errorf("Executions: %v", err)
		return
	}
	if len(page.Rows) != 0 {
		t.Errorf("Executions(unknown): got %d rows, want 0", len(page.Rows))
	}
}

// The page: the runs newest first, limit per page — page one carries
// limit rows and the HasMore flag, page two starts at the offset (the
// limit), and the union of the two pages is every run.
func TestExecutionsPagesNewestFirst(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	ids := make([]string, 8)
	for i := range ids {
		ex, err := actions.Enqueue("noinput-test", nil)
		if err != nil {
			t.Errorf("Enqueue %d: %v", i, err)
			return
		}
		ids[i] = ex.ID
	}
	for _, id := range ids {
		if _, err := testcore.PollState(t, id, status.Success); err != nil {
			t.Errorf("poll %s to success: %v", id, err)
			return
		}
	}
	first, err := reads.Executions("noinput-test", 6, 0)
	if err != nil {
		t.Errorf("Executions page one: %v", err)
		return
	}
	if len(first.Rows) != 6 {
		t.Errorf("page one: got %d rows, want 6", len(first.Rows))
		return
	}
	if !first.HasMore {
		t.Error("page one: HasMore is false, want true (eight runs, six per page)")
	}
	if first.Rows[0].ID != ids[7] {
		t.Errorf("page one first row: got %s, want the last enqueued %s", first.Rows[0].ID, ids[7])
	}
	second, err := reads.Executions("noinput-test", 6, 6)
	if err != nil {
		t.Errorf("Executions page two: %v", err)
		return
	}
	if len(second.Rows) != 2 {
		t.Errorf("page two: got %d rows, want 2", len(second.Rows))
		return
	}
	if second.HasMore {
		t.Error("page two: HasMore is true, want false")
	}
	// The union of the two pages is all the runs.
	seen := map[string]bool{}
	for _, r := range first.Rows {
		seen[r.ID] = true
	}
	for _, r := range second.Rows {
		seen[r.ID] = true
	}
	for _, id := range ids {
		if !seen[id] {
			t.Errorf("the union of the pages is missing run %s", id)
		}
	}
}

// The terminal row fields: the error row carries the error message and no
// output, the success row carries the decoded output ("ok") and no
// error, and UpdatedAt is after CreatedAt (the terminal completion time).
func TestExecutionsTerminalRowFields(t *testing.T) {
	testcore.BootTest(t, "../testworkflows")
	errEx, err := actions.Enqueue("live-test", "err")
	if err != nil {
		t.Errorf("Enqueue err: %v", err)
		return
	}
	okEx, err := actions.Enqueue("live-test", "one")
	if err != nil {
		t.Errorf("Enqueue ok: %v", err)
		return
	}
	if _, err := testcore.PollState(t, errEx.ID, status.Error); err != nil {
		t.Errorf("poll the err run to error: %v", err)
		return
	}
	if _, err := testcore.PollState(t, okEx.ID, status.Success); err != nil {
		t.Errorf("poll the ok run to success: %v", err)
		return
	}
	page, err := reads.Executions("live-test", 6, 0)
	if err != nil {
		t.Errorf("Executions: %v", err)
		return
	}
	var errRow, okRow *reads.Execution
	for i := range page.Rows {
		switch page.Rows[i].ID {
		case errEx.ID:
			errRow = &page.Rows[i]
		case okEx.ID:
			okRow = &page.Rows[i]
		}
	}
	if errRow == nil || okRow == nil {
		t.Errorf("Executions: missing the err row or the ok row (got %d rows)", len(page.Rows))
		return
	}
	if errRow.Error != "live: step failed" {
		t.Errorf("err row: Error got %q, want \"live: step failed\"", errRow.Error)
	}
	if errRow.Output != "" {
		t.Errorf("err row: Output got %q, want empty", errRow.Output)
	}
	// A fast run can finish in the created_at millisecond: the update
	// time is never before the creation.
	if errRow.UpdatedAt.Before(errRow.CreatedAt) {
		t.Errorf("err row: UpdatedAt %s is before CreatedAt %s", errRow.UpdatedAt, errRow.CreatedAt)
	}
	if okRow.Output != "ok" {
		t.Errorf("ok row: Output got %q, want ok", okRow.Output)
	}
	if okRow.Error != "" {
		t.Errorf("ok row: Error got %q, want empty", okRow.Error)
	}
	if okRow.UpdatedAt.Before(okRow.CreatedAt) {
		t.Errorf("ok row: UpdatedAt %s is before CreatedAt %s", okRow.UpdatedAt, okRow.CreatedAt)
	}
}
