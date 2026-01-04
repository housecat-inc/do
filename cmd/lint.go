package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var lintCmd = &cobra.Command{
	Use:   "lint",
	Short: "Run golangci-lint on the project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("golangci-lint"); err != nil {
			return fmt.Errorf("golangci-lint is not installed. Install it with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest")
		}

		if err := ensureLintConfig(); err != nil {
			return err
		}

		lintCmd := exec.Command("golangci-lint", "run", "./...")
		lintCmd.Stdout = os.Stdout
		lintCmd.Stderr = os.Stderr
		if err := lintCmd.Run(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				os.Exit(exitErr.ExitCode())
			}
			return fmt.Errorf("failed to run golangci-lint: %w", err)
		}

		return nil
	},
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
		return fmt.Errorf("failed to create %s: %w", configFile, err)
	}

	fmt.Printf("Created %s with default configuration\n", configFile)
	return nil
}

func init() {
	rootCmd.AddCommand(lintCmd)
}
