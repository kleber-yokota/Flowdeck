// main_e2e_test.go is the E2E seam of the process: the real binary path
// (run → boot → register → source → launch → serve) in the test process,
// the real DB (a fresh sqlite per test), the real HTTP listen, and the
// SIGTERM shutdown (the docker compose stop signal — the Go runtime
// handles it because run registered the signal handler).
package main

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	dbos "github.com/dbos-inc/dbos-transact-golang/dbos"
)

// freePort asks the kernel for a free TCP port on loopback: the E2E
// serve listens there (no fixed port — no collision with the dev box).
func freePort(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen 127.0.0.1:0: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

// setE2EEnv points the process at a fresh sqlite DB on a free port: the
// env the container sets, with the test values.
func setE2EEnv(t *testing.T, dsn, addr string) {
	t.Helper()
	t.Setenv("DBOS_APP_NAME", "flowdeck-e2e")
	t.Setenv("DBOS_SYSTEM_DATABASE_URL", dsn)
	t.Setenv("FLOWDECK_HTTP_ADDR", addr)
}

// waitForStatus polls /healthz until it returns the want code (the serve
// boots in a goroutine — the poll is the readiness gate).
func waitForStatus(t *testing.T, addr string, want int) {
	t.Helper()
	url := "http://" + addr + "/healthz"
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			code := resp.StatusCode
			resp.Body.Close()
			if code == want {
				return
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("healthz never returned %d on %s", want, url)
}

// run() with the required env missing is the early error (the config
// validation, before any boot).
func TestRunOfMissingEnvIsAnEarlyError(t *testing.T) {
	t.Setenv("DBOS_APP_NAME", "")
	t.Setenv("DBOS_SYSTEM_DATABASE_URL", "")
	if err := run(); err == nil || !strings.Contains(err.Error(), "required") {
		t.Fatalf("run with missing env: got %v, want the required-env error", err)
	}
}

// run() with an unsupported database URL fails at the explicit boot
// (the error surfaces before any launch).
func TestRunOfABadDatabaseURLIsABootError(t *testing.T) {
	t.Setenv("DBOS_APP_NAME", "flowdeck-e2e")
	t.Setenv("DBOS_SYSTEM_DATABASE_URL", "bogus://x")
	if err := run(); err == nil || !strings.Contains(err.Error(), "database") {
		t.Fatalf("run with a bad database URL: got %v, want the boot error", err)
	}
}

// run() with the HTTP port already taken fails at the listen (the serve
// error, before any request is served).
func TestRunOfAnOccupiedPortIsAListenError(t *testing.T) {
	dsn := "sqlite:" + filepath.Join(t.TempDir(), "listen.db")
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("hold the port: %v", err)
	}
	defer ln.Close()

	setE2EEnv(t, dsn, ln.Addr().String())
	if err := run(); err == nil || !strings.Contains(err.Error(), "listen") {
		t.Fatalf("run with an occupied port: got %v, want the listen error", err)
	}
}

// run() with the serial queue already owned by another application
// fails at the launch (the queue registration is the conflict — the
// boot and the source parse ran before it).
func TestRunOfAConflictingSerialQueueIsALaunchError(t *testing.T) {
	dsn := "sqlite:" + filepath.Join(t.TempDir(), "conflict.db")

	other, err := dbos.NewContext(context.Background(), dbos.Config{
		AppName:     "other-app",
		DatabaseURL: dsn,
	})
	if err != nil {
		t.Fatalf("other-app context: %v", err)
	}
	if _, err := dbos.RegisterQueue(other, "serial"); err != nil {
		t.Fatalf("other-app serial queue: %v", err)
	}
	dbos.Shutdown(other, time.Second)

	setE2EEnv(t, dsn, freePort(t))
	if err := run(); err == nil || !strings.Contains(err.Error(), "serial queue") {
		t.Fatalf("run with a conflicting serial queue: got %v, want the launch error", err)
	}
}

// run() with a working config: the real serve (the real listen, the real
// core), a ready /healthz, and the clean stop on SIGTERM (the docker
// compose stop signal — run registered the handler, so the Go runtime
// delivers it to the context, not the process exit). run, not main: the
// healthcheck flag lives in main, and redefining it in the same process
// (a -count=2 run) panics.
func TestRunOfAWorkingCoreServesAndStopsOnSIGTERM(t *testing.T) {
	dsn := "sqlite:" + filepath.Join(t.TempDir(), "serve.db")
	addr := freePort(t)
	setE2EEnv(t, dsn, addr)

	done := make(chan struct{})
	go func() {
		run()
		close(done)
	}()

	waitForStatus(t, addr, http.StatusOK)
	syscall.Kill(os.Getpid(), syscall.SIGTERM)

	select {
	case <-done:
	case <-time.After(30 * time.Second):
		t.Fatalf("run did not stop within 30s of the SIGTERM")
	}
}

// healthcheckExit: the 0 of a healthy server (the 127.0.0.1 rewrite of
// the ":port" form), the 1 of a dead port, the 1 of a non-200 /healthz,
// and the 1 of an unparseable address.
func TestHealthcheckExit(t *testing.T) {
	// The healthy server: the success exit, and the empty-host rewrite of
	// the ":port" form to 127.0.0.1.
	healthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer healthy.Close()
	port := strings.TrimPrefix(strings.TrimPrefix(healthy.URL, "http://"), "127.0.0.1:")
	if got := healthcheckExit(":" + port); got != 0 {
		t.Fatalf("healthcheck of a healthy server: got %d, want 0", got)
	}

	// The dead port: the connection failure is a 1.
	dead := freePort(t)
	if got := healthcheckExit(dead); got != 1 {
		t.Fatalf("healthcheck of a dead port: got %d, want 1", got)
	}

	// The non-200 /healthz: the failure exit.
	unhealthy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer unhealthy.Close()
	if got := healthcheckExit(strings.TrimPrefix(unhealthy.URL, "http://")); got != 1 {
		t.Fatalf("healthcheck of a 503 /healthz: got %d, want 1", got)
	}

	// The unparseable address (no port at all): the invalid-address exit.
	if got := healthcheckExit("8080"); got != 1 {
		t.Fatalf("healthcheck of a bad address: got %d, want 1", got)
	}

	// The empty address: the default ":8080" — the default is rewritten and
	// pinged; the box serves no healthy /healthz there (nothing does on
	// this dev box, and a free port refuses the connection), so the exit
	// is 1 either way (the branch is exercised for coverage of the default).
	if got := healthcheckExit(""); got != 1 {
		t.Fatalf("healthcheck of the default port: got %d, want 1", got)
	}
}
