// childpaths_test.go pins the child paths of the detail and the steps
// partial: the dedupe of repeated child ids (one path per child), the
// failed child read (the error propagates, no guess), the child lines of
// only the success rows, and the 500 of the detail page and the steps
// partial when the child's status row is gone (the real-DB gone-row
// state, no mock).
package executions

import (
	"net/http"
	"testing"
	"time"

	"flowdeck/core"
	"flowdeck/core/reads"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
	"flowdeck/web/internal/webtest"
)

// A child run with the same id on two step rows is read once (the dedupe
// of the child paths — one path per child, in first-seen order).
func TestChildPathsOfDuplicateChildIDsReadOnce(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()
	p := core.NewPort()

	parent := webtest.Ack(t, srv.URL, "child-test", nil)
	id := webtest.ExecutionID(t, parent)
	if _, err := testcore.PollState(t, id, status.Success); err != nil {
		t.Fatalf("poll: %v", err)
	}
	path, err := p.Steps(id)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	var childID string
	for _, row := range path.Steps {
		if row.Kind == reads.KindChild {
			childID = row.ChildID
			break
		}
	}
	if childID == "" {
		t.Fatalf("child-test run without a child row")
	}

	// The same child id on two rows: one path, not two.
	d := reads.DetailPath{RunPath: reads.RunPath{Steps: []reads.Step{
		{Name: "child", Kind: reads.KindChild, ChildID: childID},
		{Name: "child", Kind: reads.KindChild, ChildID: childID},
	}}}
	out, err := childPathsOf(p, d)
	if err != nil {
		t.Fatalf("childPathsOf: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("childPathsOf: got %d child paths, want 1 (the dedupe)", len(out))
	}
}

// A child id the core cannot read is an error (the child paths stop —
// the page renders no half of the children).
func TestChildPathsOfAnUnreadableChildIsAnError(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()
	p := core.NewPort()

	d := reads.DetailPath{RunPath: reads.RunPath{Steps: []reads.Step{
		{Name: "child", Kind: reads.KindChild, ChildID: "00000000-0000-0000-0000-000000000000"},
	}}}
	if _, err := childPathsOf(p, d); err == nil {
		t.Fatalf("childPathsOf of an unreadable child: got nil error, want the read error")
	}
}

// childLinesFor lists only the success rows of the child path (the
// failed rows are not lines — the child block shows the state only).
func TestChildLinesOfOnlySuccessRows(t *testing.T) {
	start := time.Now()
	cp := reads.RunPath{Steps: []reads.Step{
		{Name: "ok", State: status.Success, StartedAt: start, CompletedAt: start.Add(2 * time.Second)},
		{Name: "bad", State: status.Error, StartedAt: start, CompletedAt: start.Add(time.Second)},
	}}
	lines := childLinesFor(cp)
	if len(lines) != 1 || lines[0].Name != "ok" || lines[0].Duration != "done in 2.0s" {
		t.Fatalf("childLinesFor: got %v, want the one success row", lines)
	}
}

// The detail page with the child's status row gone under the live core:
// the child paths read fails — a clean 404 (the unknown-execution error
// of the child), not a half of the children.
func TestDetailPageOfAGoneChildIs404(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../../core/testworkflows")
	srv := testServer(t, core.NewPort())
	defer srv.Close()

	parent := webtest.Ack(t, srv.URL, "child-test", nil)
	id := webtest.ExecutionID(t, parent)
	p := core.NewPort()
	if _, err := testcore.PollState(t, id, status.Success); err != nil {
		t.Fatalf("poll: %v", err)
	}
	path, err := p.Steps(id)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	var childID string
	for _, row := range path.Steps {
		if row.Kind == reads.KindChild {
			childID = row.ChildID
			break
		}
	}
	if childID == "" {
		t.Fatalf("child-test run without a child row")
	}
	testcore.DeleteStatusRow(t, dbfile, childID)

	if code, _ := webtest.Get(t, srv.URL+"/executions/"+id); code != http.StatusNotFound {
		t.Fatalf("detail of a gone child: got %d, want %d", code, http.StatusNotFound)
	}
}

// The steps partial with the child's status row gone: the same 404 (the
// steps partial reads the child paths too).
func TestStepsPartialOfAGoneChildIs404(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../../core/testworkflows")
	srv := testServer(t, core.NewPort())
	defer srv.Close()

	parent := webtest.Ack(t, srv.URL, "child-test", nil)
	id := webtest.ExecutionID(t, parent)
	p := core.NewPort()
	if _, err := testcore.PollState(t, id, status.Success); err != nil {
		t.Fatalf("poll: %v", err)
	}
	path, err := p.Steps(id)
	if err != nil {
		t.Fatalf("Steps: %v", err)
	}
	var childID string
	for _, row := range path.Steps {
		if row.Kind == reads.KindChild {
			childID = row.ChildID
			break
		}
	}
	if childID == "" {
		t.Fatalf("child-test run without a child row")
	}
	testcore.DeleteStatusRow(t, dbfile, childID)

	if code, _ := webtest.Get(t, srv.URL+"/executions/"+id+"/steps"); code != http.StatusNotFound {
		t.Fatalf("steps of a gone child: got %d, want %d", code, http.StatusNotFound)
	}
}
