package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

const ciWorkflow = `name: CI

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Build and Test
        run: go tool do

  deploy:
    runs-on: ubuntu-latest
    needs: build
    if: github.event_name == 'pull_request' && vars.CLOUDSDK_CORE_PROJECT != ''
    permissions:
      contents: read
      id-token: write
      pull-requests: write
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Authenticate to Google Cloud
        uses: google-github-actions/auth@v2
        with:
          workload_identity_provider: ${{ vars.WORKLOAD_IDENTITY_PROVIDER }}
          service_account: ${{ vars.SERVICE_ACCOUNT }}

      - name: Set up Cloud SDK
        uses: google-github-actions/setup-gcloud@v2

      - name: Install ko
        run: go install github.com/google/ko@latest

      - name: Deploy preview
        id: deploy
        env:
          CLOUDSDK_CORE_PROJECT: ${{ vars.CLOUDSDK_CORE_PROJECT }}
          CLOUDSDK_RUN_REGION: ${{ vars.CLOUDSDK_RUN_REGION }}
          CLOUD_RUN_SERVICE: ${{ vars.CLOUD_RUN_SERVICE }}
          KO_DOCKER_REPO: gcr.io/${{ vars.CLOUDSDK_CORE_PROJECT }}/${{ vars.CLOUD_RUN_SERVICE }}
        run: |
          TAG="pr-${{ github.event.pull_request.number }}"
          go tool do deploy --tag="$TAG"

          # Get the preview URL
          URL=$(gcloud run services describe $CLOUD_RUN_SERVICE \
            --platform=managed \
            --region=$CLOUDSDK_RUN_REGION \
            --project=$CLOUDSDK_CORE_PROJECT \
            --format="value(status.traffic.url)" \
            | grep "$TAG" | head -1)
          echo "url=$URL" >> $GITHUB_OUTPUT

      - name: Comment on PR
        uses: actions/github-script@v7
        with:
          script: |
            const url = '${{ steps.deploy.outputs.url }}';
            if (!url) return;

            const body = '## Preview Deploy\n\n' + url;
            const { data: comments } = await github.rest.issues.listComments({
              owner: context.repo.owner,
              repo: context.repo.repo,
              issue_number: context.issue.number,
            });

            const existing = comments.find(c => c.body.includes('## Preview Deploy'));
            if (existing) {
              await github.rest.issues.updateComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                comment_id: existing.id,
                body,
              });
            } else {
              await github.rest.issues.createComment({
                owner: context.repo.owner,
                repo: context.repo.repo,
                issue_number: context.issue.number,
                body,
              });
            }
`

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Create GitHub Actions CI workflow",
	Long: `Creates a .github/workflows/ci.yml that:
- Runs 'go tool do' on all pushes and PRs
- Deploys preview environments for PRs (if GCP vars are configured)
- Comments the preview URL on the PR

Required GitHub repository variables for deploy:
  CLOUDSDK_CORE_PROJECT      - GCP project ID
  CLOUDSDK_RUN_REGION        - Cloud Run region
  CLOUD_RUN_SERVICE          - Cloud Run service name
  WORKLOAD_IDENTITY_PROVIDER - Workload identity provider
  SERVICE_ACCOUNT            - Service account email`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Find project root
		root, err := findProjectRoot()
		if err != nil {
			return err
		}

		// Create .github/workflows directory
		workflowDir := filepath.Join(root, ".github", "workflows")
		if err := os.MkdirAll(workflowDir, 0755); err != nil {
			return errors.WithStack(err)
		}

		// Write workflow file
		workflowPath := filepath.Join(workflowDir, "ci.yml")
		if err := os.WriteFile(workflowPath, []byte(ciWorkflow), 0644); err != nil {
			return errors.WithStack(err)
		}

		fmt.Printf("Created %s\n", workflowPath)
		fmt.Println("\nTo enable preview deploys, configure these repository variables:")
		fmt.Println("  CLOUDSDK_CORE_PROJECT")
		fmt.Println("  CLOUDSDK_RUN_REGION")
		fmt.Println("  CLOUD_RUN_SERVICE")
		fmt.Println("  WORKLOAD_IDENTITY_PROVIDER")
		fmt.Println("  SERVICE_ACCOUNT")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ciCmd)
}
