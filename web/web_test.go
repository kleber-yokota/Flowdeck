package web

import (
	"net/http/httptest"
	"os"
	"testing"

	"flowdeck/core"
	"flowdeck/core/boot"
	"flowdeck/core/testcore"
	"flowdeck/web/internal/webtest"
)

// bootedTestServer builds the full-stack server of a booted core (no
// stubs): the per-test boot (testcore.BootTest — the fresh sqlite file
// and the testworkflows fixtures), and New wires every route.
func bootedTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	testcore.BootTest(t, "../core/testworkflows")
	return webtest.TestServer(t, New(core.NewPort(), boot.Ready))
}

func TestMain(m *testing.M) { os.Exit(testcore.TestMain(m, "../core/testworkflows")) }
