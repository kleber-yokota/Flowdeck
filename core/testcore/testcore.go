// Package testcore serves the core · web · workflows test binaries: the
// per-test boot (BootTest — one explicit boot per test, on a fresh sqlite
// file under the test's TempDir), the shared poll helpers, and the shared
// TestMain body (TestMain — the source dir only, no boot). Every booted
// test boots its own core and shuts it down (the t.Cleanup): there is no
// env boot and no skip — the default state is not-booted.
package testcore

import (
	"context"
	"database/sql"
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
// (the TempDir removal is the framework's). The dir is the workflowsDir
// (relative to the test binary's directory); the register funcs, when
// given, replace the full registration (the ten test workflows) with a
// named subset on top of the fixture queues.
//
// Tests read the live context from the return — no env, no shared DB.
func BootTest(t *testing.T, dir string, register ...func(dbos.Context)) dbos.Context {
	t.Helper()
	c, _ := BootTestDB(t, dir, register...)
	return c
}

// BootTestDB is BootTest with the fresh sqlite file path returned: the
// read-failure contract tests reach the file itself (a second connection
// drops a schema table — DropTable).
func BootTestDB(t *testing.T, dir string, register ...func(dbos.Context)) (dbos.Context, string) {
	t.Helper()
	dbfile := path.Join(t.TempDir(), "test.db")
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+dbfile); err != nil {
		t.Fatalf("boot: %v", err)
	}
	c := boot.Ctx()
	if err := boot.SetWorkflowsSource(os.DirFS(dir)); err != nil {
		t.Fatalf("set workflows source: %v", err)
	}
	if len(register) == 0 {
		testworkflows.RegisterTestWorkflows(c)
	} else {
		testworkflows.RegisterQueues(c)
		for _, r := range register {
			r(c)
		}
	}
	if err := boot.Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}
	t.Cleanup(func() { boot.Shutdown(5 * time.Second) })
	return c, dbfile
}

// secondConnDSN is the DSN of the second sqlite connection: the busy
// timeout lets the drop wait out the booted core's in-flight writes
// (the live executor polls and claims while the schema is manipulated).
const secondConnDSN = "?_pragma=busy_timeout(10000)"

// OpenSecondConn opens the second sqlite connection on the booted file:
// the busy timeout lets the write wait out the booted core's in-flight
// writes (the live executor polls and claims while the schema or rows are
// manipulated). The cleanup closes it.
func OpenSecondConn(t *testing.T, dbfile string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", dbfile+secondConnDSN)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// SetUnmappedState writes a state the SDK cannot produce into the run's
// status row through a second sqlite connection: the read must reject the
// unmapped state, not guess it.
func SetUnmappedState(t *testing.T, dbfile, id string) {
	t.Helper()
	conn := OpenSecondConn(t, dbfile)
	if _, err := conn.Exec("UPDATE workflow_status SET status = 'BOGUS' WHERE workflow_uuid = ?", id); err != nil {
		t.Fatalf("set unmapped state: %v", err)
	}
}

// DropTable drops one schema table through a second sqlite connection on
// the booted file: the live connection's next read re-reads the schema
// and fails (the real-DB read-failure state, no mock). The booted test's
// fresh DB makes the drop harmless.
func DropTable(t *testing.T, dbfile, table string) {
	t.Helper()
	conn := OpenSecondConn(t, dbfile)
	if _, err := conn.Exec("DROP TABLE " + table); err != nil {
		t.Fatalf("drop %s: %v", table, err)
	}
}

// DeleteStatusRow deletes one run's status row through a second sqlite
// connection on the booted file: the run is gone — its reads return the
// unknown-execution error (the real-DB gone-row state, no mock). The run
// must be terminal (no live executor writes while the row is gone).
func DeleteStatusRow(t *testing.T, dbfile, id string) {
	t.Helper()
	conn := OpenSecondConn(t, dbfile)
	if _, err := conn.Exec("DELETE FROM workflow_status WHERE workflow_uuid = ?", id); err != nil {
		t.Fatalf("delete status row %s: %v", id, err)
	}
}

// TestMain is the shared TestMain body: it sets the workflows source
// (source dir only — no boot, no env: the bootless tests need no launch,
// and every booted test boots per-test via BootTest) and runs the tests.
func TestMain(m *testing.M, dir string) int {
	if err := boot.SetWorkflowsSource(os.DirFS(dir)); err != nil {
		fmt.Fprintln(os.Stderr, "set workflows source:", err)
		return 1
	}
	return m.Run()
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
