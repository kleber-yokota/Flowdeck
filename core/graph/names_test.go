package graph

import (
	"go/ast"
	"testing"
)

// runCall finds the first RunAsStep call of the snippet's body.
func runCall(t *testing.T, src string) *ast.CallExpr {
	t.Helper()
	body, err := parseSnippet("func wf(ctx c) {\n" + src + "\n}")
	if err != nil {
		t.Fatalf("parseSnippet: %v", err)
	}
	var call *ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok && calleeName(c.Fun) == "RunAsStep" {
			call = c
			return false
		}
		return true
	})
	if call == nil {
		t.Fatalf("no RunAsStep call in the snippet")
	}
	return call
}

// The step name is the WithStepName literal when present (the pinned
// name, not the func name).
func TestRunAsStepNameUsesTheStepNameOption(t *testing.T) {
	call := runCall(t, "_ = dbos.RunAsStep(ctx, step(), dbos.WithStepName(\"one\"))")
	if got := runAsStepName(call); got != "one" {
		t.Errorf("runAsStepName: got %q, want one", got)
	}
}

// Without the option, the step name is the step function's name — the
// bare ident (GreetStep) and the selector (pkg.Step) both resolve to the
// final name.
func TestRunAsStepNameFallsBackToTheStepFuncName(t *testing.T) {
	ident := runCall(t, "_ = dbos.RunAsStep(ctx, GreetStep(msg))")
	if got := runAsStepName(ident); got != "GreetStep" {
		t.Errorf("runAsStepName(ident): got %q, want GreetStep", got)
	}
	sel := runCall(t, "_ = dbos.RunAsStep(ctx, pkg.Step())")
	if got := runAsStepName(sel); got != "Step" {
		t.Errorf("runAsStepName(selector): got %q, want Step", got)
	}
}

// No option and no step arg: the empty name (the call is not a named
// step).
func TestRunAsStepNameEmptyWithoutOptionOrArg(t *testing.T) {
	call := runCall(t, "_ = dbos.RunAsStep(ctx)")
	if got := runAsStepName(call); got != "" {
		t.Errorf("runAsStepName: got %q, want empty", got)
	}
}

// A non-literal option arg is not a name — the option is absent, and the
// caller falls back to the func name.
func TestOptionNameRejectsANonLiteralArg(t *testing.T) {
	call := runCall(t, "_ = dbos.RunAsStep(ctx, step(), dbos.WithStepName(nameVar))")
	if name, ok := optionName(call, "WithStepName"); ok || name != "" {
		t.Errorf("optionName(non-literal): got %q/%v, want absent", name, ok)
	}
	if got := runAsStepName(call); got != "step" {
		t.Errorf("runAsStepName(fallback): got %q, want step", got)
	}
}

// An option that is not present is reported absent.
func TestOptionNameAbsent(t *testing.T) {
	call := runCall(t, "_ = dbos.RunAsStep(ctx, step())")
	if name, ok := optionName(call, "WithStepName"); ok || name != "" {
		t.Errorf("optionName(absent): got %q/%v, want absent", name, ok)
	}
}

// A call whose callee is neither an ident nor a selector (a paren) has
// no name.
func TestCalleeNameEmptyForANonCallee(t *testing.T) {
	paren := &ast.ParenExpr{X: &ast.Ident{Name: "x"}}
	if got := calleeName(paren); got != "" {
		t.Errorf("calleeName(paren): got %q, want empty", got)
	}
}

// exprName resolves the nested call's callee (f() → f's name) and is
// empty for an expression that has no name (a composite literal).
func TestExprNameNestedCallAndComposite(t *testing.T) {
	ident := runCall(t, "_ = dbos.RunAsStep(ctx, step())")
	if got := exprName(ident.Args[1]); got != "step" {
		t.Errorf("exprName(call): got %q, want step", got)
	}
	if got := exprName(&ast.CompositeLit{}); got != "" {
		t.Errorf("exprName(composite): got %q, want empty", got)
	}
}
