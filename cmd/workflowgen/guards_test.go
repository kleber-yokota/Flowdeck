// workflowgen_guards_test.go pins the guards of the generator: main()
// returning cleanly on a good module (the generated file written, no
// exit), the module walk up to a parent go.mod, the walk failure of an
// unreadable tree, the parse failure of a broken Go file, and the
// register-decl rejections (method, wrong name, a result, extra params,
// a non-Context param).
package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// run() without a go.mod above the working directory: the module name
// lookup fails (the walk reaches the filesystem root) — the error is
// returned, no file is written.
func TestRunWithoutAModuleIsAnError(t *testing.T) {
	inTempModule(t, map[string]string{"workflows.go": "package workflows\n"})
	if _, err := os.Stat("zz_generated.go"); err == nil {
		t.Fatal("run wrote a file without a module")
	}
	if err := run(); err == nil {
		t.Fatal("run without a module: got nil error, want the module error")
	} else if !strings.Contains(err.Error(), "no go.mod") {
		t.Fatalf("run without a module: got %v, want the no-go.mod error", err)
	}
}

// run() with a Go file the parser cannot read: the subpackage scan fails
// (the parse error propagates — no file is written).
func TestRunWithABrokenGoFileIsAnError(t *testing.T) {
	inTempModule(t, map[string]string{
		"go.mod":    "module flowdeck\n",
		"broken.go": "package broken\n\nfunc ( {\n",
	})
	if err := run(); err == nil {
		t.Fatal("run with a broken Go file: got nil error, want the parse error")
	}
	if _, err := os.Stat("zz_generated.go"); err == nil {
		t.Fatal("run wrote a file with a broken Go file")
	}
}

// main() on a good module: the generated file is written to the working
// directory and main returns (no exit — the error branch only exits).
func TestMainOfAGoodModuleWritesAndReturns(t *testing.T) {
	inTempModule(t, map[string]string{
		"go.mod":     "module flowdeck\n",
		"buy/buy.go": "package buy\n\nfunc Register(c dbos.Context) {}\n",
	})
	main()
	if _, err := os.Stat("zz_generated.go"); err != nil {
		t.Fatalf("zz_generated.go not written: %v", err)
	}
}

// moduleName walks up to the nearest go.mod: from a subdirectory without
// its own go.mod, the parent's module name is the answer.
func TestModuleNameWalksUpToAParent(t *testing.T) {
	inTempModule(t, map[string]string{
		"go.mod": "module flowdeck\n",
		"sub/README.md": "no go.mod here\n",
	})
	if err := os.Chdir(filepath.Join(".", "sub")); err != nil {
		t.Fatalf("chdir sub: %v", err)
	}
	name, err := moduleName()
	if err != nil {
		t.Fatalf("moduleName from a sub: %v", err)
	}
	if name != "flowdeck" {
		t.Fatalf("moduleName from a sub: got %q, want flowdeck", name)
	}
}

// subpackages of an unreadable tree: the walk error propagates (no
// guessing which subpackages exist).
func TestSubpackagesOfAnUnwalkableTreeIsAnError(t *testing.T) {
	inTempModule(t, map[string]string{
		"go.mod": "module flowdeck\n",
	})
	locked := filepath.Join(".", "locked")
	if err := os.Mkdir(locked, 0o755); err != nil {
		t.Fatalf("mkdir locked: %v", err)
	}
	if err := os.WriteFile(filepath.Join(locked, "x.go"), []byte("package x\n"), 0o644); err != nil {
		t.Fatalf("write x.go: %v", err)
	}
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatalf("chmod locked: %v", err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })

	if _, err := subpackages("."); err == nil {
		t.Fatal("subpackages of an unreadable tree: got nil error, want the walk error")
	}
}

// subpackages with a Go file the parser cannot read: the parse error
// propagates (the generator does not skip broken sources).
func TestSubpackagesOfABrokenGoFileIsAnError(t *testing.T) {
	inTempModule(t, map[string]string{
		"go.mod":   "module flowdeck\n",
		"broken.go": "package broken\n\nfunc ( {\n",
	})
	if _, err := subpackages("."); err == nil {
		t.Fatal("subpackages with a broken Go file: got nil error, want the parse error")
	}
}

// parseDecls parses one source snippet into its declarations: the
// register-decl rejections are unit-tested against parsed declarations.
func parseDecls(t *testing.T, src string) []ast.Decl {
	t.Helper()
	f, err := parser.ParseFile(token.NewFileSet(), "x.go", src, 0)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	return f.Decls
}

// registerDecl rejects every declaration that is not the registration:
// a non-func decl, a method, a wrong name, a result, extra params, and a
// param type that is not dbos.Context.
func TestRegisterDeclRejections(t *testing.T) {
	decls := parseDecls(t, `
package x

var v int

type T struct{}

func (t T) Register(c dbos.Context) {}

func Other(c dbos.Context) {}

func Register(c dbos.Context) int { return 0 }

func Register(c dbos.Context, d dbos.Context) {}

func Register(c int) {}

func Register(x dbos.Context) {}
`)
	for i, decl := range decls {
		if registerDecl(decl) {
			t.Fatalf("registerDecl accepted decl %d that is not the registration", i)
		}
	}
}

// The positive control: the registration itself is accepted (the
// rejections above are not a blanket rejection).
func TestRegisterDeclOfTheRegistration(t *testing.T) {
	decls := parseDecls(t, `
package x

func Register(c dbos.Context) {}
`)
	if len(decls) != 1 || !registerDecl(decls[0]) {
		t.Fatal("registerDecl rejected the registration itself")
	}
}
