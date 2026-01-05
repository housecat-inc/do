package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

var verbose bool

var rootCmd = &cobra.Command{
	Use:   "do",
	Short: "A CLI tool for app init, build, test, deploy",
	RunE: func(cmd *cobra.Command, args []string) error {
		type command struct {
			args       []string
			hasVerbose bool
		}

		commands := []command{
			{[]string{"go", "generate", "./..."}, true},
			{[]string{"go", "build", "./..."}, true},
			{[]string{"go", "vet", "./..."}, false},
			{[]string{"go", "tool", "do", "lint"}, false},
			{[]string{"go", "test", "./..."}, true},
		}

		for _, c := range commands {
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

func init() {
	rootCmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
