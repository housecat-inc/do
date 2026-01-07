package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/housecat-inc/do/pkg/gcloud"
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
            | tr ';' '\n' | grep "$TAG" | head -1)
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

  deploy-prod:
    runs-on: ubuntu-latest
    needs: build
    if: github.event_name == 'push' && github.ref == 'refs/heads/main' && vars.CLOUDSDK_CORE_PROJECT != ''
    permissions:
      contents: read
      id-token: write
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

      - name: Deploy to production
        env:
          CLOUDSDK_CORE_PROJECT: ${{ vars.CLOUDSDK_CORE_PROJECT }}
          CLOUDSDK_RUN_REGION: ${{ vars.CLOUDSDK_RUN_REGION }}
          CLOUD_RUN_SERVICE: ${{ vars.CLOUD_RUN_SERVICE }}
          KO_DOCKER_REPO: gcr.io/${{ vars.CLOUDSDK_CORE_PROJECT }}/${{ vars.CLOUD_RUN_SERVICE }}
        run: go tool do deploy
`

var ciSetup bool

var ciCmd = &cobra.Command{
	Use:   "ci",
	Short: "Create GitHub Actions CI workflow",
	Long: `Creates a .github/workflows/ci.yml that:
- Runs 'go tool do' on all pushes and PRs
- Deploys preview environments for PRs (if GCP vars are configured)
- Comments the preview URL on the PR
- Deploys to production on merge to main

Use --setup to configure GCP Workload Identity Federation for CI deploys.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if ciSetup {
			return runCISetup()
		}

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
		fmt.Println("\nRun 'go tool do ci --setup' to configure GCP Workload Identity for deploys.")
		return nil
	},
}

func runCISetup() error {
	// Get project from environment
	project := os.Getenv("CLOUDSDK_CORE_PROJECT")
	if project == "" {
		return errors.New("CLOUDSDK_CORE_PROJECT not set. Run 'go tool do deploy' first to configure.")
	}

	region := os.Getenv("CLOUDSDK_RUN_REGION")
	if region == "" {
		region = "us-central1"
	}

	service := os.Getenv("CLOUD_RUN_SERVICE")
	if service == "" {
		return errors.New("CLOUD_RUN_SERVICE not set. Run 'go tool do deploy' first to configure.")
	}

	// Get repo from git remote
	var out bytes.Buffer
	gitCmd := exec.Command("git", "remote", "get-url", "origin")
	gitCmd.Stdout = &out
	if err := gitCmd.Run(); err != nil {
		return errors.New("failed to get git remote. Make sure you're in a git repo with a remote.")
	}

	remote := strings.TrimSpace(out.String())
	repo := extractGitHubRepo(remote)
	if repo == "" {
		return errors.Errorf("could not parse GitHub repo from remote: %s", remote)
	}

	fmt.Printf("Setting up CI for %s (project: %s)\n\n", repo, project)

	// Check gcloud is installed and authenticated
	if !gcloud.IsInstalled() {
		return errors.New("gcloud is not installed")
	}
	if !gcloud.IsAuthenticated() {
		return errors.New("gcloud is not authenticated. Run 'gcloud auth login'")
	}

	// Enable required APIs
	fmt.Println("Enabling required APIs...")
	if err := gcloud.Run("gcloud", "services", "enable",
		"iamcredentials.googleapis.com",
		"run.googleapis.com",
		"artifactregistry.googleapis.com",
		"--project="+project); err != nil {
		return err
	}

	// Create workload identity pool (ignore error if exists)
	fmt.Println("\nCreating workload identity pool...")
	_ = gcloud.Run("gcloud", "iam", "workload-identity-pools", "create", "github",
		"--project="+project,
		"--location=global",
		"--display-name=GitHub Actions")

	// Create OIDC provider (ignore error if exists)
	fmt.Println("\nCreating OIDC provider...")
	_ = gcloud.Run("gcloud", "iam", "workload-identity-pools", "providers", "create-oidc", "github",
		"--project="+project,
		"--location=global",
		"--workload-identity-pool=github",
		"--display-name=GitHub",
		"--attribute-mapping=google.subject=assertion.sub,attribute.actor=assertion.actor,attribute.repository=assertion.repository",
		"--attribute-condition=assertion.repository=='"+repo+"'",
		"--issuer-uri=https://token.actions.githubusercontent.com")

	// Create service account (ignore error if exists)
	fmt.Println("\nCreating service account...")
	_ = gcloud.Run("gcloud", "iam", "service-accounts", "create", "github-actions",
		"--project="+project,
		"--display-name=GitHub Actions")

	serviceAccount := fmt.Sprintf("github-actions@%s.iam.gserviceaccount.com", project)

	// Grant roles
	fmt.Println("\nGranting IAM roles...")
	roles := []string{"roles/run.admin", "roles/storage.admin", "roles/artifactregistry.writer"}
	for _, role := range roles {
		if err := gcloud.Run("gcloud", "projects", "add-iam-policy-binding", project,
			"--member=serviceAccount:"+serviceAccount,
			"--role="+role); err != nil {
			return err
		}
	}

	// Get project number for compute service account
	var numOut bytes.Buffer
	numCmd := exec.Command("gcloud", "projects", "describe", project, "--format=value(projectNumber)")
	numCmd.Stdout = &numOut
	if err := numCmd.Run(); err != nil {
		return errors.WithStack(err)
	}
	projectNumber := strings.TrimSpace(numOut.String())

	// Allow github-actions to act as compute service account
	computeSA := fmt.Sprintf("%s-compute@developer.gserviceaccount.com", projectNumber)
	if err := gcloud.Run("gcloud", "iam", "service-accounts", "add-iam-policy-binding", computeSA,
		"--member=serviceAccount:"+serviceAccount,
		"--role=roles/iam.serviceAccountUser",
		"--project="+project); err != nil {
		return err
	}

	// Allow workload identity to impersonate service account
	member := fmt.Sprintf("principalSet://iam.googleapis.com/projects/%s/locations/global/workloadIdentityPools/github/attribute.repository/%s",
		projectNumber, repo)
	if err := gcloud.Run("gcloud", "iam", "service-accounts", "add-iam-policy-binding", serviceAccount,
		"--project="+project,
		"--role=roles/iam.workloadIdentityUser",
		"--member="+member); err != nil {
		return err
	}

	// Print GitHub variables
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Add these to GitHub: Settings > Secrets and variables > Actions > Variables")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("\nCLOUDSDK_CORE_PROJECT=%s\n", project)
	fmt.Printf("CLOUDSDK_RUN_REGION=%s\n", region)
	fmt.Printf("CLOUD_RUN_SERVICE=%s\n", service)
	fmt.Printf("WORKLOAD_IDENTITY_PROVIDER=projects/%s/locations/global/workloadIdentityPools/github/providers/github\n", projectNumber)
	fmt.Printf("SERVICE_ACCOUNT=%s\n", serviceAccount)

	return nil
}

func extractGitHubRepo(remote string) string {
	// Handle SSH: git@github.com:owner/repo.git
	if strings.HasPrefix(remote, "git@github.com:") {
		repo := strings.TrimPrefix(remote, "git@github.com:")
		repo = strings.TrimSuffix(repo, ".git")
		return repo
	}
	// Handle HTTPS: https://github.com/owner/repo.git
	if strings.Contains(remote, "github.com/") {
		parts := strings.Split(remote, "github.com/")
		if len(parts) == 2 {
			repo := strings.TrimSuffix(parts[1], ".git")
			return repo
		}
	}
	return ""
}

func init() {
	ciCmd.Flags().BoolVar(&ciSetup, "setup", false, "configure GCP Workload Identity Federation for CI deploys")
	rootCmd.AddCommand(ciCmd)
}
