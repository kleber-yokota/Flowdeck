package boot_test

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"

	"flowdeck/core/boot"
)

// freshFile is a fresh sqlite file in the test's temp dir.
func freshFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "test.sqlite")
}

// notBooted makes sure the singleton is down (a previous test's shutdown
// leaves it down — the guard is defensive, the tests run in order).
func notBooted(t *testing.T) {
	t.Helper()
	if boot.Ctx() != nil {
		boot.Shutdown(time.Second)
	}
}

// A not-launched core: Launch needs a Boot first (the explicit-boot
// contract — no implicit env boot).
func TestLaunchNeedsABootedCore(t *testing.T) {
	notBooted(t)
	err := boot.Launch()
	if err == nil || err.Error() != "core: Launch needs a booted core (Boot)" {
		t.Fatalf("Launch unbooted: got %v, want the needs-a-boot error", err)
	}
}

// The singleton holds one executor: a second Launch without a Shutdown
// in between is rejected.
func TestLaunchTwiceIsRejected(t *testing.T) {
	notBooted(t)
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+freshFile(t)); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	defer boot.Shutdown(time.Second)
	if err := boot.Launch(); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if err := boot.Launch(); err == nil || !strings.Contains(err.Error(), "Launch already called") {
		t.Fatalf("second Launch: got %v, want the already-called error", err)
	}
}

// A boot failure is returned to the caller, not logged: a database URL
// the SDK cannot open (an empty URL) fails at the Boot.
func TestBootRejectsABadDatabaseURL(t *testing.T) {
	err := boot.Boot(context.Background(), "flowdeck_test", "")
	if err == nil || !strings.Contains(err.Error(), "dbos boot") {
		t.Fatalf("Boot with empty URL: got %v, want the boot error", err)
	}
}

// failFS is an unreadable source: the paths build fails (the error is
// returned, not logged).
type failFS struct{}

func (failFS) Open(name string) (fs.File, error) {
	return nil, errors.New("unreadable")
}

// SetWorkflowsSource of an unreadable source is an error (the build
// paths fails — no partial graph).
func TestSetWorkflowsSourceRejectsAnUnreadableSource(t *testing.T) {
	if err := boot.SetWorkflowsSource(failFS{}); err == nil {
		t.Fatal("SetWorkflowsSource of an unreadable source: got nil, want an error")
	}
}

// secondConn opens the booted file through a fresh connection (the live
// core's next launch step re-reads the schema after the manipulation).
func secondConn(t *testing.T, dbfile string) *sql.DB {
	t.Helper()
	conn, err := sql.Open("sqlite", dbfile+"?_pragma=busy_timeout(10000)")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

// reboots boots the core on a fresh file, launches it (creating the
// schema), shuts it down and boots again on the same file — the second
// boot is the one whose Launch meets the manipulated schema.
func reboots(t *testing.T) string {
	t.Helper()
	file := freshFile(t)
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+file); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	if err := boot.Launch(); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	boot.Shutdown(time.Second)
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+file); err != nil {
		t.Fatalf("second Boot: %v", err)
	}
	return file
}

// The launch registers the queues: with the queues table dropped, the
// main queue's registration fails (the launch stops — the executor is
// not started on a broken schema).
func TestLaunchFailsWhenTheQueuesTableIsMissing(t *testing.T) {
	file := reboots(t)
	defer boot.Shutdown(time.Second)
	if _, err := secondConn(t, file).Exec("DROP TABLE queues"); err != nil {
		t.Fatalf("drop queues: %v", err)
	}
	if err := boot.Launch(); err == nil || !strings.Contains(err.Error(), "register queue") {
		t.Fatalf("Launch with queues missing: got %v, want the register-queue error", err)
	}
}

// The serial queue is owned by the process's application: with the row
// owned by another application, the serial registration fails (queue
// names are unique across applications sharing a system database).
func TestLaunchFailsWhenTheSerialQueueIsForeign(t *testing.T) {
	file := reboots(t)
	defer boot.Shutdown(time.Second)
	if _, err := secondConn(t, file).Exec("UPDATE queues SET application_name = 'other-app' WHERE name = 'serial'"); err != nil {
		t.Fatalf("re-own serial queue: %v", err)
	}
	if err := boot.Launch(); err == nil || !strings.Contains(err.Error(), "register serial queue") {
		t.Fatalf("Launch with a foreign serial queue: got %v, want the register-serial-queue error", err)
	}
}

// The launch recovers the pending workflows: with the status table
// dropped, the recovery fails (the executor is not started on a broken
// schema).
func TestLaunchFailsWhenPendingRecoveryCannotRead(t *testing.T) {
	file := reboots(t)
	defer boot.Shutdown(time.Second)
	if _, err := secondConn(t, file).Exec("DROP TABLE workflow_status"); err != nil {
		t.Fatalf("drop workflow_status: %v", err)
	}
	if err := boot.Launch(); err == nil || !strings.Contains(err.Error(), "dbos launch") {
		t.Fatalf("Launch with pending recovery broken: got %v, want the launch error", err)
	}
}

// The read contacts of a not-launched core: CtxOrErr and Ready return
// ErrNotLaunched, Paths is nil (the web maps the 503).
func TestReadContactsOfAnUnbootedCore(t *testing.T) {
	notBooted(t)
	if _, err := boot.CtxOrErr(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("CtxOrErr unbooted: got %v, want ErrNotLaunched", err)
	}
	if err := boot.Ready(); !errors.Is(err, boot.ErrNotLaunched) {
		t.Fatalf("Ready unbooted: got %v, want ErrNotLaunched", err)
	}
	if boot.Paths() != nil {
		t.Fatal("Paths unbooted: got a graph, want nil")
	}
}

// A Shutdown of a core that was never booted is a no-op (the singleton
// is reset, nothing closes).
func TestShutdownOfAnUnbootedCore(t *testing.T) {
	notBooted(t)
	boot.Shutdown(time.Second)
	if boot.Ctx() != nil {
		t.Fatal("Shutdown unbooted: the context should stay nil")
	}
}

// The read contacts of a booted-and-launched core: CtxOrErr returns the
// live context, Ready is nil.
func TestReadContactsOfALaunchedCore(t *testing.T) {
	notBooted(t)
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+freshFile(t)); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	defer boot.Shutdown(time.Second)
	if err := boot.Launch(); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	if c, err := boot.CtxOrErr(); err != nil || c == nil {
		t.Fatalf("CtxOrErr launched: got %v, want the live context", err)
	}
	if err := boot.Ready(); err != nil {
		t.Fatalf("Ready launched: got %v, want nil", err)
	}
}

// warnLog collects the slog records of the test's duration: the launch
// warns (not errors) when a source-declared workflow is not registered.
type warnLog struct {
	buf []string
}

func (w *warnLog) Enabled(context.Context, slog.Level) bool { return true }

func (w *warnLog) Handle(_ context.Context, r slog.Record) error {
	w.buf = append(w.buf, r.Message)
	return nil
}

func (w *warnLog) WithAttrs([]slog.Attr) slog.Handler { return w }

func (w *warnLog) WithGroup(string) slog.Handler { return w }

// unnamedProbeWf is the registry entry without a workflow name: its
// registry name falls back to the FQN (the source match uses it).
func unnamedProbeWf(ctx dbos.Context, msg string) (string, error) {
	return msg, nil
}

// namedProbeWf is the single source-matching registration of the
// warning test.
func namedProbeWf(ctx dbos.Context, msg string) (string, error) {
	return msg, nil
}

// The launch compares the source-declared workflows with the registry:
// a name the source declares but the registry lacks warns (the generated
// registration is stale) — the launch itself succeeds.
func TestLaunchWarnsAboutUnregisteredSourceWorkflows(t *testing.T) {
	notBooted(t)
	if err := boot.Boot(context.Background(), "flowdeck_test", "sqlite:"+freshFile(t)); err != nil {
		t.Fatalf("Boot: %v", err)
	}
	defer boot.Shutdown(time.Second)
	if err := boot.SetWorkflowsSource(os.DirFS(filepath.Join("..", "..", "workflows"))); err != nil {
		t.Fatalf("SetWorkflowsSource: %v", err)
	}
	w := &warnLog{}
	prev := slog.Default().Handler()
	slog.SetDefault(slog.New(w))
	defer slog.SetDefault(slog.New(prev))
	c := boot.Ctx()
	dbos.RegisterWorkflow(c, unnamedProbeWf)
	dbos.RegisterWorkflow(c, namedProbeWf, dbos.WithWorkflowName("probe-test"))
	if err := boot.Launch(); err != nil {
		t.Fatalf("Launch: %v", err)
	}
	found := false
	for _, b := range w.buf {
		if strings.Contains(b, "workflow declared in source but not registered") {
			found = true
		}
	}
	if !found {
		t.Fatalf("launch warnings: got %v, want the unregistered-source warning", w.buf)
	}
}
