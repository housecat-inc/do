package nocomments

import (
	"go/ast"
	"go/token"
	"strings"

	doanalysis "github.com/housecat-inc/do/pkg/analysis"
	"golang.org/x/tools/go/analysis"
)

const (
	MsgNoComments doanalysis.Message = "write self-commenting code; use //! prefix if truly important"
)

var Analyzer = &doanalysis.Analyzer{
	Analyzer: &analysis.Analyzer{
		Name: "nocomments",
		Doc:  "disallows comments except godoc and //! for important notes",
		Run:  run,
	},
	Messages: []doanalysis.Message{MsgNoComments},
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		docPositions := collectDocPositions(file)

		for _, cg := range file.Comments {
			for _, c := range cg.List {
				if isAllowed(c, docPositions) {
					continue
				}
				MsgNoComments.Report(pass, c.Pos())
			}
		}
	}
	return nil, nil
}

func collectDocPositions(file *ast.File) map[token.Pos]bool {
	docs := make(map[token.Pos]bool)

	if file.Doc != nil {
		for _, c := range file.Doc.List {
			docs[c.Pos()] = true
		}
	}

	ast.Inspect(file, func(n ast.Node) bool {
		var doc *ast.CommentGroup
		switch x := n.(type) {
		case *ast.GenDecl:
			doc = x.Doc
		case *ast.FuncDecl:
			doc = x.Doc
		case *ast.TypeSpec:
			doc = x.Doc
		case *ast.ValueSpec:
			doc = x.Doc
		case *ast.Field:
			doc = x.Doc
		}
		if doc != nil {
			for _, c := range doc.List {
				docs[c.Pos()] = true
			}
		}
		return true
	})

	return docs
}

func isAllowed(c *ast.Comment, docPositions map[token.Pos]bool) bool {
	if docPositions[c.Pos()] {
		return true
	}

	text := c.Text
	if strings.HasPrefix(text, "//!") {
		return true
	}
	if strings.HasPrefix(text, "/*!") {
		return true
	}

	return false
}
