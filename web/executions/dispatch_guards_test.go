// dispatch_guards_test.go pins the dispatch guards: a malformed form body
// is a 400 (the form never reaches the spec), and a value that does not
// sanitize against the spec is a 400 (the web sanitizes, the core trusts
// the caller — the enqueue never sees the bad input).
package executions

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

// A POST whose body is not valid urlencoded form data is a 400 (the form
// parse fails before the spec is consulted).
func TestDispatchOfAMalformedFormBodyIs400(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/run/noinput-test", "application/x-www-form-urlencoded", strings.NewReader("Count=%zz"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed form body: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if !strings.Contains(string(body), "form:") {
		t.Fatalf("malformed form body: body %q, want the form error", string(body))
	}
}

// A form value that does not convert to the input's type is a 400: the
// int field of the struct input receives a non-int (the enqueue never
// sees it — the payload is rejected before the core).
func TestDispatchOfANonIntValueIs400(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/run/struct-test", "application/x-www-form-urlencoded", strings.NewReader("Count=abc"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("non-int value: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if !strings.Contains(string(body), "input:") {
		t.Fatalf("non-int value: body %q, want the input error", string(body))
	}
}

// A form field the spec does not know is a 400 (no extra fields — the
// spec is the closed list of inputs).
func TestDispatchOfAnUnknownFieldIs400(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/run/noinput-test", "application/x-www-form-urlencoded", strings.NewReader("Count=1"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("unknown field: got %d, want %d", resp.StatusCode, http.StatusBadRequest)
	}
	if !strings.Contains(string(body), "input:") {
		t.Fatalf("unknown field: body %q, want the input error", string(body))
	}
}
