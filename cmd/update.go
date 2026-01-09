package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var directFlag bool

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update do to the latest version",
	RunE: func(cmd *cobra.Command, args []string) error {
		goCmd := exec.Command("go", "get", "-tool", "github.com/housecat-inc/do@main")
		goCmd.Stdout = os.Stdout
		goCmd.Stderr = os.Stderr

		if directFlag {
			goCmd.Env = append(os.Environ(), "GOPROXY=direct")
			return runWithRetry(goCmd, 3, 2*time.Second)
		}

		if err := goCmd.Run(); err != nil {
			return errors.WithStack(err)
		}
		return nil
	},
}

func runWithRetry(cmd *exec.Cmd, maxRetries int, delay time.Duration) error {
	var lastErr error
	for i := range maxRetries {
		if i > 0 {
			fmt.Printf("Retrying (%d/%d)...\n", i+1, maxRetries)
			// Re-create the command since exec.Cmd can only be run once
			cmd = exec.Command(cmd.Args[0], cmd.Args[1:]...)
			cmd.Env = append(os.Environ(), "GOPROXY=direct")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			time.Sleep(delay)
		}
		if err := cmd.Run(); err != nil {
			lastErr = err
			continue
		}
		return nil
	}
	return errors.WithStack(lastErr)
}

func init() {
	updateCmd.Flags().BoolVarP(&directFlag, "direct", "d", false, "bypass Go proxy to get latest commit (with retries)")
	rootCmd.AddCommand(updateCmd)
}
