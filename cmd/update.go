package cmd

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update do to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		goCmd := exec.Command("go", "get", "-tool", "github.com/housecat-inc/do@latest")
		goCmd.Stdout = os.Stdout
		goCmd.Stderr = os.Stderr
		if err := goCmd.Run(); err != nil {
			return errors.WithStack(err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
