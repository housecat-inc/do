package analysis

import (
	"go/token"

	"golang.org/x/tools/go/analysis"
)

type Message string

func (m Message) Report(pass *analysis.Pass, pos token.Pos) {
	pass.Reportf(pos, "%s", m)
}

type Analyzer struct {
	*analysis.Analyzer
	Messages []Message
}
