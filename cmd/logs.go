package cmd

import (
	"os"
	"os/exec"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var logsTail bool

var logsCmd = &cobra.Command{
	Use:   "logs",
	Short: "View logs from the deployed Cloud Run service",
	RunE: func(cmd *cobra.Command, args []string) error {
		project := os.Getenv("CLOUDSDK_CORE_PROJECT")
		region := os.Getenv("CLOUDSDK_RUN_REGION")
		service := os.Getenv("CLOUD_RUN_SERVICE")

		if project == "" || region == "" || service == "" {
			return errors.New("no service deployed. Run 'go do deploy' first")
		}

		var run *exec.Cmd
		if logsTail {
			// Use gcloud beta run services logs tail for live streaming
			run = exec.Command("gcloud", "beta", "run", "services", "logs", "tail", service,
				"--project="+project,
				"--region="+region)
		} else {
			// Use gcloud beta run services logs read for recent logs
			run = exec.Command("gcloud", "beta", "run", "services", "logs", "read", service,
				"--project="+project,
				"--region="+region)
		}
		run.Stdout = os.Stdout
		run.Stderr = os.Stderr
		run.Stdin = os.Stdin

		if err := run.Run(); err != nil {
			return errors.WithStack(err)
		}

		return nil
	},
}

func init() {
	logsCmd.Flags().BoolVarP(&logsTail, "tail", "t", false, "Tail logs in real-time")
	rootCmd.AddCommand(logsCmd)
}
