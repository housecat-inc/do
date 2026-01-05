package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/housecat-inc/do/pkg/analysis/pkgerrors"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Run linters on the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("golangci-lint"); err != nil {
			return errors.New("golangci-lint is not installed. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
		}

		if err := ensureLintConfig(); err != nil {
			return err
		}

		var hasErrors bool

		// Run golangci-lint
		golangci := exec.Command("golangci-lint", "run", "./...")
		golangci.Stdout = os.Stdout
		golangci.Stderr = os.Stderr
		if err := golangci.Run(); err != nil {
			hasErrors = true
		}

		// Run custom analyzers
		if issues := runAnalyzers("./...", pkgerrors.Analyzer); issues > 0 {
			hasErrors = true
		}

		if hasErrors {
			os.Exit(1)
		}
		return nil
	},
}

func runAnalyzers(pattern string, analyzers ...*analysis.Analyzer) int {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
	}

	pkgs, err := packages.Load(cfg, pattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load packages: %v\n", err)
		return 1
	}

	var issues int
	for _, pkg := range pkgs {
		for _, a := range analyzers {
			pass := &analysis.Pass{
				Analyzer:  a,
				Fset:      pkg.Fset,
				Files:     pkg.Syntax,
				Pkg:       pkg.Types,
				TypesInfo: pkg.TypesInfo,
				Report: func(d analysis.Diagnostic) {
					pos := pkg.Fset.Position(d.Pos)
					fmt.Fprintf(os.Stderr, "%s: %s (%s)\n", pos, d.Message, a.Name)
					issues++
				},
			}
			_, _ = a.Run(pass)
		}
	}
	return issues
}

func ensureLintConfig() error {
	const configFile = ".golangci.yml"

	if _, err := os.Stat(configFile); err == nil {
		return nil
	}

	const defaultConfig = `version: "2"

linters:
  enable:
    - errcheck
    - govet
    - ineffassign
    - staticcheck
    - unused
`

	if err := os.WriteFile(configFile, []byte(defaultConfig), 0644); err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("Created %s with default configuration\n", configFile)
	return nil
}

func init() {
	rootCmd.AddCommand(lintCmd)
}
