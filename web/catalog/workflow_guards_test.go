// workflow_guards_test.go pins the guards of the workflow page: the pager
// clamps the page below 1 (the first page), the unknown workflow is a
// clean 404 (the registry, no DB), and the unreadable executions list is
// a 500 (the status table dropped under the live core — the spec and the
// graph come from the source, only the runlines touch the DB).
package catalog

import (
	"net/http"
	"testing"

	"flowdeck/core"
	"flowdeck/core/testcore"
	"flowdeck/web/internal/webtest"
)

// execPager clamps the page below 1 to the first page: the range shown is
// from the start of the listing, and there is no prev link.
func TestExecPagerClampsANegativePage(t *testing.T) {
	start, end, prev, next := execPager(0, 3, true)
	if start != 1 || end != 3 || prev != 0 || next != 2 {
		t.Fatalf("execPager(0, 3, true): got %d %d %d %d, want 1 3 0 2", start, end, prev, next)
	}
	start, end, prev, next = execPager(-2, 0, false)
	if start != 1 || end != 0 || prev != 0 || next != 0 {
		t.Fatalf("execPager(-2, 0, false): got %d %d %d %d, want 1 0 0 0", start, end, prev, next)
	}
}

// The workflow page of a name the registry does not know is a clean 404:
// the spec lookup fails before the DB is touched.
func TestWorkflowPageOfAnUnknownWorkflowIs404(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	if code, _ := webtest.Get(t, srv.URL+"/workflows/nope"); code != http.StatusNotFound {
		t.Fatalf("workflow page of an unknown workflow: got %d, want %d", code, http.StatusNotFound)
	}
}

// The workflow page with the status table dropped under the live core: the
// spec and the paths graph render from the source (no DB), but the runlines
// read fails — a 500, not a guess.
func TestWorkflowPageOfAnUnreadableStatusTableIs500(t *testing.T) {
	_, dbfile := testcore.BootTestDB(t, "../../core/testworkflows")
	srv := testServer(t, core.NewPort())
	defer srv.Close()

	testcore.DropTable(t, dbfile, "workflow_status")
	if code, _ := webtest.Get(t, srv.URL+"/workflows/linear-test"); code != http.StatusInternalServerError {
		t.Fatalf("workflow page with the status table dropped: got %d, want %d", code, http.StatusInternalServerError)
	}
}
