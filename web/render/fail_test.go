package render

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"flowdeck/core/actions"
	"flowdeck/core/boot"
	"flowdeck/core/reads"
	"flowdeck/core/registry"
)

// Fail maps a core error onto the HTTP error response: each sentinel is
// its own case (the wrapped forms carry the sentinel — errors.Is), and
// anything unexpected is a logged 500.
func failCase(t *testing.T, err error, wantCode int, wantBody string) {
	t.Helper()
	rec := httptest.NewRecorder()
	Fail(rec, "op", err)
	if rec.Code != wantCode {
		t.Fatalf("status: got %d want %d", rec.Code, wantCode)
	}
	if !strings.Contains(rec.Body.String(), wantBody) {
		t.Fatalf("body: got %q, want %q", rec.Body.String(), wantBody)
	}
}

func TestFailOfAnUnknownWorkflowIs404(t *testing.T) {
	failCase(t, registry.ErrUnknownWorkflow, http.StatusNotFound, "unknown workflow")
}

func TestFailOfAnUnknownExecutionIs404(t *testing.T) {
	failCase(t, reads.ErrUnknownExecution, http.StatusNotFound, "unknown execution")
}

func TestFailOfANotResumableRunIs400(t *testing.T) {
	failCase(t, actions.ErrNotResumable, http.StatusBadRequest, "run cannot be resumed")
}

func TestFailOfANotCancellableRunIs400(t *testing.T) {
	failCase(t, actions.ErrNotCancellable, http.StatusBadRequest, "run cannot be cancelled")
}

func TestFailOfANotLaunchedCoreIs503(t *testing.T) {
	failCase(t, boot.ErrNotLaunched, http.StatusServiceUnavailable, "core not ready")
}

func TestFailOfAnUnexpectedErrorIs500(t *testing.T) {
	failCase(t, errors.New("boom"), http.StatusInternalServerError, "internal error")
}
