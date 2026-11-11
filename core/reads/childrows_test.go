package reads

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
)

// bootedNoWorkflows boots a fresh core with no workflow registration:
// the childRows read needs only the live DBOS context and the schema.
func bootedNoWorkflows(t *testing.T) dbosCtxAndFile {
	t.Helper()
	file := filepath.Join(t.TempDir(), "test.db")
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+file); err != nil {
		t.Fatalf("boot: %v", err)
	}
	t.Cleanup(func() { boot.Shutdown(5 * time.Second) })
	if err := boot.Launch(); err != nil {
		t.Fatalf("launch: %v", err)
	}
	return dbosCtxAndFile{ctx: boot.Ctx(), file: file}
}

type dbosCtxAndFile struct {
	ctx  dbos.Context
	file string
}

// The child-rows read is the batched query by the child ids: with the
// status table dropped (a second connection — the real DB, no mock), the
// query fails and the read returns the wrapped error.
func TestChildRowsReadFailure(t *testing.T) {
	bf := bootedNoWorkflows(t)
	conn, err := sql.Open("sqlite", bf.file+"?_pragma=busy_timeout(10000)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := conn.Exec("DROP TABLE workflow_status"); err != nil {
		t.Fatalf("drop workflow_status: %v", err)
	}
	if _, err := childRows(bf.ctx, []string{"no-such-child"}); err == nil {
		t.Fatal("childRows after the dropped status table: got nil, want a read failure")
	}
}
