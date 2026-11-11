package core

import (
	"errors"
	"testing"
	"time"

	"flowdeck/core/boot"
	"flowdeck/core/registry"
	"flowdeck/core/status"
	"flowdeck/core/testcore"
)

// ensureUnbooted makes sure the singleton is down (the previous test's
// shutdown leaves it down — the guard is defensive, the tests run in
// order).
func ensureUnbooted(t *testing.T) {
	t.Helper()
	if boot.Ctx() != nil {
		boot.Shutdown(time.Second)
	}
}

// The port of a not-launched core: the catalog reads are empty or
// unknown (no registry; the TestMain source is set, so the graph is the
// source graph and the unknown spec is an error), and every run, dispatch
// and board read returns ErrNotLaunched (the web maps the 503).
func TestPortOfAnUnbootedCore(t *testing.T) {
	ensureUnbooted(t)
	p := NewPort()

	if got := p.List(); got != nil {
		t.Fatalf("List unbooted: got %d workflows, want none", len(got))
	}
	if g := p.Graph(); g == nil || g.Of("linear-test") == nil {
		t.Fatalf("Graph unbooted: got %v, want the source graph with linear-test", g)
	}
	if _, err := p.Spec("nope"); !errors.Is(err, registry.ErrUnknownWorkflow) {
		t.Fatalf("Spec unbooted: got %v, want ErrUnknownWorkflow", err)
	}

	reads := []func() error{
		func() error { _, err := p.Status("x"); return err },
		func() error { _, err := p.Steps("x"); return err },
		func() error { _, err := p.Detail("x"); return err },
		func() error { _, err := p.StepDetail("x", "s"); return err },
		func() error { _, err := p.Executions("x", 5, 0); return err },
		func() error { _, err := p.Enqueue("x", 1); return err },
		func() error { _, err := p.Resume("x"); return err },
		func() error { _, err := p.Cancel("x"); return err },
		func() error { _, err := p.Counts(); return err },
		func() error { _, err := p.Living(); return err },
		func() error { _, err := p.Queued(); return err },
		func() error { _, err := p.Failed(); return err },
	}
	for i, f := range reads {
		if err := f(); err == nil || err != boot.ErrNotLaunched {
			t.Fatalf("port method %d unbooted: got %v, want ErrNotLaunched", i, err)
		}
	}
}

// The port of a booted-and-launched core delegates to the live core:
// the catalog (list, graph, spec), the runs (status, steps, detail,
// step detail, executions), the dispatch (enqueue, cancel, resume) and
// the live board (counts, living, queued, failed).
func TestPortOfABootedCore(t *testing.T) {
	_, _ = testcore.BootTestDB(t, "testworkflows")
	p := NewPort()

	// The catalog: the ten test workflows, the paths graph, the spec.
	ws := p.List()
	if len(ws) != 10 {
		t.Fatalf("List: got %d workflows, want 10", len(ws))
	}
	if g := p.Graph(); g == nil || g.Of("linear-test") == nil {
		t.Fatal("Graph: got nil or the graph without linear-test")
	}
	if _, err := p.Spec("linear-test"); err != nil {
		t.Fatalf("Spec linear-test: %v", err)
	}
	if _, err := p.Spec("nope"); !errors.Is(err, registry.ErrUnknownWorkflow) {
		t.Fatalf("Spec of the unknown: got %v, want ErrUnknownWorkflow", err)
	}

	// A run through every read: linear-test succeeds.
	e, err := p.Enqueue("linear-test", "hi")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	id := e.ID
	if err := waitForState(p, id, status.Success); err != nil {
		t.Fatalf("waiting for success: %v", err)
	}
	if st, err := p.Status(id); err != nil || st != status.Success {
		t.Fatalf("Status: got %v, %v — want success", st, err)
	}
	path, err := p.Steps(id)
	if err != nil || path.State != status.Success || len(path.Steps) == 0 {
		t.Fatalf("Steps: got %v, %v — want the success path", path, err)
	}
	d, err := p.Detail(id)
	if err != nil || d.RunPath.State != status.Success {
		t.Fatalf("Detail: got %v, %v — want the success detail", d, err)
	}
	if _, err := p.StepDetail(id, "fetch"); err != nil {
		t.Fatalf("StepDetail: %v", err)
	}
	page, err := p.Executions("linear-test", 10, 0)
	if err != nil || len(page.Rows) == 0 {
		t.Fatalf("Executions: got %v, %v — want the page", page, err)
	}

	// The dispatch: a blocked run is cancelled, then resumed (the
	// cancelled run is re-enqueued on the same id).
	b, err := p.Enqueue("block-test", 1)
	if err != nil {
		t.Fatalf("Enqueue block-test: %v", err)
	}
	if _, err := p.Cancel(b.ID); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if st, err := p.Status(b.ID); err != nil || st != status.Cancelled {
		t.Fatalf("Status after cancel: got %v, %v — want cancelled", st, err)
	}
	if _, err := p.Resume(b.ID); err != nil {
		t.Fatalf("Resume: %v", err)
	}

	// The live board: a failed run (failing-test) is in the failed
	// bucket with its message; the counters are the live read.
	f, err := p.Enqueue("failing-test", 1)
	if err != nil {
		t.Fatalf("Enqueue failing-test: %v", err)
	}
	if err := waitForState(p, f.ID, status.Error); err != nil {
		t.Fatalf("waiting for the failure: %v", err)
	}
	counts, err := p.Counts()
	if err != nil {
		t.Fatalf("Counts: %v", err)
	}
	if counts.Failed == 0 {
		t.Fatal("Counts: the failed bucket is empty after a failure")
	}
	if _, err := p.Living(); err != nil {
		t.Fatalf("Living: %v", err)
	}
	if _, err := p.Queued(); err != nil {
		t.Fatalf("Queued: %v", err)
	}
	failed, err := p.Failed()
	if err != nil {
		t.Fatalf("Failed: %v", err)
	}
	if len(failed) == 0 || failed[0].Message == "" {
		t.Fatalf("Failed: got %v — want the failing-test row with its message", failed)
	}
}

// After a shutdown the paths graph is reset: the port's Graph falls back
// to the empty graph (Of returns nil — the web renders the empty catalog).
func TestPortGraphAfterShutdownIsEmpty(t *testing.T) {
	ensureUnbooted(t)
	if g := NewPort().Graph(); g == nil || g.Of("linear-test") != nil {
		t.Fatalf("Graph after shutdown: got %v, want the empty graph", g)
	}
}

// waitForState polls the port's Status until the state (or an error).
func waitForState(p Port, id string, want status.State) error {
	for i := 0; i < 100; i++ {
		st, err := p.Status(id)
		if err != nil {
			return err
		}
		if st == want {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("timeout waiting for state " + string(want))
}
