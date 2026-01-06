package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show deployment status and service info",
	RunE: func(cmd *cobra.Command, args []string) error {
		project := os.Getenv("CLOUDSDK_CORE_PROJECT")
		region := os.Getenv("CLOUDSDK_RUN_REGION")
		service := os.Getenv("CLOUD_RUN_SERVICE")

		if project == "" || region == "" || service == "" {
			return errors.New("no service deployed. Run 'go do deploy' first")
		}

		fmt.Printf("Project: %s\n", project)
		fmt.Printf("Region:  %s\n", region)
		fmt.Printf("Service: %s\n", service)

		// Get service details
		run := exec.Command("gcloud", "run", "services", "describe", service,
			"--project="+project,
			"--region="+region,
			"--format=value(status.url,status.latestReadyRevisionName)")
		out, err := run.Output()
		if err != nil {
			return errors.Wrap(err, "failed to get service status")
		}

		parts := strings.Split(strings.TrimSpace(string(out)), "\n")
		if len(parts) >= 1 && parts[0] != "" {
			fmt.Printf("URL:     %s\n", parts[0])
		}
		if len(parts) >= 2 && parts[1] != "" {
			fmt.Printf("Latest:  %s\n", parts[1])
		}

		// Get traffic tags
		run = exec.Command("gcloud", "run", "services", "describe", service,
			"--project="+project,
			"--region="+region,
			"--format=json(status.traffic)")
		out, err = run.Output()
		if err == nil {
			outStr := string(out)
			if strings.Contains(outStr, "tag") {
				fmt.Println("\nTraffic tags:")
				// Parse and show tags with URLs
				run = exec.Command("gcloud", "run", "services", "describe", service,
					"--project="+project,
					"--region="+region,
					"--format=table(status.traffic.tag,status.traffic.percent,status.traffic.url)")
				run.Stdout = os.Stdout
				run.Stderr = os.Stderr
				_ = run.Run()
			}
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
