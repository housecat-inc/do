package cmd

import (
	"fmt"
	"go/ast"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	doanalysis "github.com/housecat-inc/do/pkg/analysis"
	"github.com/housecat-inc/do/pkg/analysis/nocomments"
	"github.com/housecat-inc/do/pkg/analysis/pkgerrors"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/packages"
)

var listAnalyzers bool

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Run linters on the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		analyzers := []*doanalysis.Analyzer{pkgerrors.Analyzer, nocomments.Analyzer}

		if listAnalyzers {
			for _, a := range analyzers {
				fmt.Printf("%s: %s\n", a.Name, a.Doc)
				for _, msg := range a.Messages {
					fmt.Printf("  - %s\n", msg)
				}
			}
			return nil
		}

		if _, err := exec.LookPath("golangci-lint"); err != nil {
			install := exec.Command("go", "install", "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest")
			install.Stdout = os.Stdout
			install.Stderr = os.Stderr
			if err := install.Run(); err != nil {
				return errors.WithStack(err)
			}
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
		if issues := runAnalyzers("./...", analyzers); issues > 0 {
			hasErrors = true
		}

		if hasErrors {
			os.Exit(1)
		}
		return nil
	},
}

func runAnalyzers(pattern string, analyzers []*doanalysis.Analyzer) int {
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
		files := filterGenerated(pkg.Syntax)
		if len(files) == 0 {
			continue
		}

		for _, a := range analyzers {
			pass := &analysis.Pass{
				Analyzer:  a.Analyzer,
				Fset:      pkg.Fset,
				Files:     files,
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

func filterGenerated(files []*ast.File) []*ast.File {
	var result []*ast.File
	for _, f := range files {
		if !isGenerated(f) {
			result = append(result, f)
		}
	}
	return result
}

func isGenerated(file *ast.File) bool {
	for _, cg := range file.Comments {
		for _, c := range cg.List {
			if strings.HasPrefix(c.Text, "// Code generated") && strings.HasSuffix(c.Text, "DO NOT EDIT.") {
				return true
			}
		}
	}
	return false
}

func ensureLintConfig() error {
	// Find project root (where go.mod is)
	root, err := findProjectRoot()
	if err != nil {
		return err
	}

	configFile := filepath.Join(root, ".golangci.yml")

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

func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", errors.WithStack(err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find go.mod")
		}
		dir = parent
	}
}

func init() {
	lintCmd.Flags().BoolVarP(&listAnalyzers, "list", "l", false, "list custom analyzers and their descriptions")
	rootCmd.AddCommand(lintCmd)
}
