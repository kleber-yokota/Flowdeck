package core_test

import (
	"os"
	"testing"

	"flowdeck/core/internal/testcore"
)

// TestMain brings the env-driven core up for the tests: the paths graph
// from the test workflows' source (the testcore package directory — the
// parser fixture, the twin of main.go's //go:embed workflows), the test
// workflows and the executor (only when the env context exists — the unit
// tests run without it).
func TestMain(m *testing.M) {
	testcore.Setup("testworkflows")
	os.Exit(m.Run())
}
