package pkgerrors

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Name: "pkgerrors",
	Doc:  "checks that github.com/pkg/errors is used instead of the standard errors package or fmt.Errorf",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// Check for "errors" import
		for _, imp := range file.Imports {
			if imp.Path.Value == `"errors"` {
				pass.Reportf(imp.Pos(), "use github.com/pkg/errors instead of the standard errors package")
			}
		}

		// Check for fmt.Errorf calls
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			ident, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			if ident.Name == "fmt" && sel.Sel.Name == "Errorf" {
				pass.Reportf(call.Pos(), "use github.com/pkg/errors errors.WithStack by default and errors.Wrap only if it will be unwrapped")
			}
			return true
		})
	}
	return nil, nil
}
