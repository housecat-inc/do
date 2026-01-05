package pkgerrors

import (
	"go/ast"

	doanalysis "github.com/housecat-inc/do/pkg/analysis"
	"golang.org/x/tools/go/analysis"
)

const (
	MsgFmtErrorf doanalysis.Message = "use github.com/pkg/errors errors.WithStack by default and errors.Wrap only if it will be unwrapped"
)

var Analyzer = &doanalysis.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name: "pkgerrors",
		Doc:  "checks that github.com/pkg/errors is used instead of the standard errors package or fmt.Errorf",
		Run:  run,
	},
	Messages: []doanalysis.Message{MsgFmtErrorf},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		for _, imp := range file.Imports {
			if imp.Path.Value == `"errors"` {
				MsgFmtErrorf.Report(pass, imp.Pos())
			}
		}

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
				MsgFmtErrorf.Report(pass, call.Pos())
			}
			return true
		})
	}
	return nil, nil
}
