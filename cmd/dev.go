package cmd

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "Run development server with live reload",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("air"); err != nil {
			install := exec.Command("go", "get", "-tool", "github.com/air-verse/air@latest")
			install.Stdout = os.Stdout
			install.Stderr = os.Stderr
			if err := install.Run(); err != nil {
				return errors.WithStack(err)
			}
		}

		air := exec.Command("air",
			"--tmp_dir", "bin",
			"--build.pre_cmd", "go generate ./...",
			"--build.cmd", "go build -o bin/app ./cmd/app",
			"--build.bin", "bin/app",
			"--build.exclude_dir", "node_modules,bin,vendor,.git,dist,build",
			"--build.exclude_regex", `\.min\.js$|\.sql\.go$|_templ\.go$|_test\.go$|out\.css$|pkg/db/(db|models|querier)\.go$`,
			"--build.include_ext", "css,go,html,svelte,templ",
		)
		air.Stdout = os.Stdout
		air.Stderr = os.Stderr
		air.Stdin = os.Stdin
		if err := air.Run(); err != nil {
			return errors.WithStack(err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(devCmd)
}
