package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "do",
	Short: "A CLI tool for app init, build, test, deploy",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip CI setup for certain commands
		if cmd.Name() == "help" || cmd.Name() == "init" {
			return nil
		}
		return ciSetupIfNeeded()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Install templ if needed (for go generate)
		if _, err := exec.LookPath("templ"); err != nil {
			install := exec.Command("go", "install", "github.com/a-h/templ/cmd/templ@latest")
			install.Stdout = os.Stdout
			install.Stderr = os.Stderr
			if err := install.Run(); err != nil {
				// Ignore error - templ may not be needed
				_ = err
			}
		}

		type command struct {
			args       []string
			hasVerbose bool
			skipInCI   bool
		}

		commands := []command{
			{[]string{"go", "generate", "./..."}, true, false},
			{[]string{"go", "mod", "tidy"}, true, true},
			{[]string{"go", "build", "-o", "/dev/null", "./..."}, true, false},
			{[]string{"go", "vet", "./..."}, false, false},
			{[]string{"go", "tool", "do", "lint"}, false, false},
			{[]string{"go", "test", "./..."}, true, false},
		}

		isCI := os.Getenv("CI") == "true"
		for _, c := range commands {
			if c.skipInCI && isCI {
				continue
			}
			args := c.args
			if verbose && c.hasVerbose {
				args = append(args[:2:2], append([]string{"-v"}, args[2:]...)...)
			}

			fmt.Printf(" →")
			for _, arg := range args {
				fmt.Printf(" %s", arg)
			}
			fmt.Println()

			run := exec.Command(args[0], args[1:]...)
			run.Stdout = os.Stdout
			run.Stderr = os.Stderr
			if err := run.Run(); err != nil {
				return err
			}
		}
		return nil
	},
}

// ciSetupIfNeeded runs CI-specific setup when CI=true
func ciSetupIfNeeded() error {
	if os.Getenv("CI") != "true" {
		return nil
	}

	// Skip if already run (e.g., when lint subcommand is spawned as external process)
	if os.Getenv("DO_CI_SETUP_DONE") == "1" {
		return nil
	}
	os.Setenv("DO_CI_SETUP_DONE", "1")

	// Drop local replace directives
	if err := dropLocalReplaces(); err != nil {
		return err
	}

	// Install tool dependencies
	if err := installToolDeps(); err != nil {
		return err
	}

	return nil
}

// dropLocalReplaces removes local path replace directives from go.mod
func dropLocalReplaces() error {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		return nil // No go.mod, skip
	}

	var replaces []string
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Match: replace github.com/foo => ../local/path
		if strings.HasPrefix(line, "replace ") && (strings.Contains(line, " => ../") || strings.Contains(line, " => ./")) {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				replaces = append(replaces, parts[1])
			}
		}
	}

	for _, mod := range replaces {
		fmt.Printf(" → go mod edit -dropreplace %s\n", mod)
		cmd := exec.Command("go", "mod", "edit", "-dropreplace", mod)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	// Run go mod download if we dropped any replaces
	if len(replaces) > 0 {
		fmt.Println(" → go mod download")
		cmd := exec.Command("go", "mod", "download")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

// installToolDeps installs tool dependencies from go.mod at their pinned versions
func installToolDeps() error {
	fmt.Println(" → go install tool")
	cmd := exec.Command("go", "install", "tool")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Ignore error - may have no tool dependencies
		return nil
	}
	return nil
}

func init() {
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
