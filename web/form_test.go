package web

import (
	"net/http"
	"strings"
	"testing"

	"flowdeck/web/internal/webtest"
)

// The top of the workflow page is the runline form — one item per input
// (the param span and the input, the default value of the source
// prefilled), from the spec the core parsed at boot.
func TestWorkflowPageShowsParameters(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	code, body := webtest.Get(t, srv.URL+"/workflows/linear-test")
	if code != http.StatusOK {
		t.Errorf("linear-test page: %d", code)
		return
	}
	for _, want := range []string{
		`class="runline"`,
		`hx-post="/run/linear-test"`,
		`class="param">msg<`,
		`name="msg"`,
		`value="FlowDeck"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page does not contain %q", want)
		}
	}
	// The form is on top: before the executions list.
	form := strings.Index(body, "<form")
	list := strings.Index(body, `id="executions"`)
	if form < 0 || list < 0 || form > list {
		t.Errorf("form not on top: form %d, list %d", form, list)
	}
}

// The workflow page has no step-defaults list under the form — the chips
// duplicated the step name and the literal (the same word twice: "fetch
// fetch" on the linear-test page).
func TestWorkflowPageHasNoStepDefaultsList(t *testing.T) {
	srv := bootedTestServer(t)
	defer srv.Close()

	for _, name := range []string{"linear-test", "fork-test"} {
		code, body := webtest.Get(t, srv.URL+"/workflows/"+name)
		if code != http.StatusOK {
			t.Errorf("%s page: %d", name, code)
			continue
		}
		if strings.Contains(body, `class="step-defaults"`) || strings.Contains(body, `<span class="step-name">`) {
			t.Errorf("%s page carries the step-defaults list", name)
		}
	}
}
