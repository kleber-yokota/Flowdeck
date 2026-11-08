package catalog

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/testcore"
	"flowdeck/web/internal/webtest"
)

// testServer builds the httptest server from the given Port: this
// package's routes on the mux (no middleware — the middlewares are the
// web package's and are tested there). The tests use the real port
// (core.NewPort): booted (bootedTestServer) or not launched (the 503 and
// empty-state paths).
func testServer(t *testing.T, p core.Port) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux, p)
	return webtest.TestServer(t, mux)
}

// bootedTestServer builds the server of a booted core (no stubs): this
// package's routes against the live core — the per-test boot
// (testcore.BootTest: the fresh sqlite file, the testworkflows fixtures,
// and the executor the boot launched).
func bootedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	testcore.BootTest(t, "../../core/testworkflows")
	return testServer(t, core.NewPort())
}

func TestMain(m *testing.M) { os.Exit(testcore.TestMain(m, "../../core/testworkflows")) }
