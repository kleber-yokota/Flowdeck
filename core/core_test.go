package core_test

import (
	"os"
	"testing"

	"flowdeck/core/testcore"
)

func TestMain(m *testing.M) { os.Exit(testcore.TestMain(m, "testworkflows")) }
