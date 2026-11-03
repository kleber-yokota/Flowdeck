package core_test

import (
	"os"
	"testing"

	"flowdeck/core/internal/testcore"

	// The workflow files register themselves in the env-driven DBOS
	// context (their init) — the same single blank import main.go carries;
	// workflows/zz_generated.go pulls in every subpackage.
	_ "flowdeck/workflows"
)

// TestMain brings the env-driven core up for the tests: the paths graph
// from the workflows tree on disk (the unit twin of main.go's
// //go:embed workflows), the test workflows (failing-test · resume-test)
// and the executor (only when the env context exists — the mock tests run
// without it).
func TestMain(m *testing.M) {
	testcore.Setup("../workflows")
	os.Exit(m.Run())
}
