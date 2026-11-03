package graph

import (
	"go/ast"
	"go/token"
	"strconv"
)

// calleeName is the call's function name, no matter the import alias:
// dbos.RunAsStep and dbostx.RunAsStep both give "RunAsStep".
func calleeName(fun ast.Expr) string {
	switch f := fun.(type) {
	case *ast.Ident:
		return f.Name
	case *ast.SelectorExpr:
		return f.Sel.Name
	}
	return ""
}

// optionName returns the string literal of the WithXxxName("...") option of
// the call, or false when the option is not present.
func optionName(call *ast.CallExpr, option string) (string, bool) {
	for _, arg := range call.Args {
		opt, isOpt := arg.(*ast.CallExpr)
		if !isOpt || calleeName(opt.Fun) != option || len(opt.Args) != 1 {
			continue
		}
		lit, isLit := opt.Args[0].(*ast.BasicLit)
		if !isLit || lit.Kind != token.STRING {
			continue
		}
		name, _ := strconv.Unquote(lit.Value)
		return name, true
	}
	return "", false
}

// runAsStepName is the step name: the WithStepName literal when present,
// otherwise the step function's name (GreetStep(msg) → "GreetStep").
func runAsStepName(call *ast.CallExpr) string {
	if name, ok := optionName(call, "WithStepName"); ok {
		return name
	}
	if len(call.Args) >= 2 {
		return exprName(call.Args[1])
	}
	return ""
}

func exprName(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return e.Sel.Name
	case *ast.CallExpr:
		return exprName(e.Fun)
	}
	return ""
}
