package cmd

import (
	"fmt"
	"os"
	"os/exec"

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
			return fmt.Errorf("failed to update: %w", err)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(updateCmd)
}
