package cmd

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/housecat-inc/do/pkg/gcloud"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var deployTag string
var deleteTag string

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy to Google Cloud Run using ko",
	Long: `Deploy to Google Cloud Run using ko.

Use --tag for branch deploys which creates a separate URL without affecting production traffic:
  go do deploy --tag=feature-x

This creates a URL like: https://feature-x---service-xxx.run.app

Use --delete-tag to remove a traffic tag:
  go do deploy --delete-tag=feature-x`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Handle tag deletion
		if deleteTag != "" {
			return deleteTrafficTag(deleteTag)
		}

		// Check required tools
		if err := checkDeployTools(); err != nil {
			return err
		}

		// Ensure authenticated with gcloud
		if !gcloud.IsAuthenticated() {
			fmt.Println("Not authenticated with Google Cloud. Starting login...")
			if err := gcloud.Login(); err != nil {
				return err
			}
		}

		// Get or select project
		project, err := selectProject()
		if err != nil {
			return err
		}

		// Get or select region
		region, err := selectRegion()
		if err != nil {
			return err
		}

		// Get or create Cloud Run service
		service, err := selectOrCreateService(project, region)
		if err != nil {
			return err
		}

		// Get build path
		buildPath, err := selectBuildPath()
		if err != nil {
			return err
		}

		// Save settings to .envrc
		if err := saveDeploySettings(project, region, service, buildPath); err != nil {
			return err
		}

		// Build and deploy with ko
		if err := deployWithKo(project, region, service, buildPath, deployTag); err != nil {
			return err
		}

		return nil
	},
}

func checkDeployTools() error {
	var missing []string

	if _, err := exec.LookPath("ko"); err != nil {
		missing = append(missing, "ko")
	}
	if !gcloud.IsInstalled() {
		missing = append(missing, "gcloud")
	}

	if len(missing) > 0 {
		return errors.Errorf("required tools not installed: %s\nInstall ko: go install github.com/ko-build/ko@latest\nInstall gcloud: https://cloud.google.com/sdk/docs/install", strings.Join(missing, ", "))
	}

	return nil
}

func selectProject() (string, error) {
	// Check if already set in environment
	if project := os.Getenv("CLOUDSDK_CORE_PROJECT"); project != "" {
		return project, nil
	}

	// Check current gcloud config
	if current := gcloud.CurrentProject(); current != "" {
		fmt.Printf("Current project: %s\n", current)
		if confirm("Use this project?") {
			return current, nil
		}
	}

	// List available projects
	fmt.Println("Fetching available projects...")
	projects, err := gcloud.ListProjects()
	if err != nil {
		return "", err
	}

	if len(projects) == 0 {
		fmt.Println("No projects found. Creating a new project...")
		return createProject()
	}

	fmt.Println("\nAvailable projects:")
	for i, p := range projects {
		fmt.Printf("  %d) %s (%s)\n", i+1, p.ID, p.Name)
	}
	fmt.Printf("  %d) Create new project\n", len(projects)+1)

	choice := promptInt("Select project", 1, len(projects)+1)
	if choice == len(projects)+1 {
		return createProject()
	}

	return projects[choice-1].ID, nil
}

func createProject() (string, error) {
	projectID := prompt("Enter new project ID")
	if projectID == "" {
		return "", errors.New("project ID cannot be empty")
	}

	fmt.Printf("Creating project %s...\n", projectID)
	if err := gcloud.CreateProject(projectID); err != nil {
		return "", err
	}

	return projectID, nil
}

func selectRegion() (string, error) {
	// Check if already set in environment
	if region := os.Getenv("CLOUDSDK_RUN_REGION"); region != "" {
		return region, nil
	}

	// Common Cloud Run regions
	regions := []string{
		"us-central1",
		"us-east1",
		"us-west1",
		"europe-west1",
		"asia-east1",
	}

	fmt.Println("\nAvailable regions:")
	for i, r := range regions {
		fmt.Printf("  %d) %s\n", i+1, r)
	}

	choice := promptInt("Select region", 1, len(regions))
	return regions[choice-1], nil
}

func selectBuildPath() (string, error) {
	// Check if already set in environment
	if path := os.Getenv("KO_BUILD_PATH"); path != "" {
		return path, nil
	}

	// Look for common patterns
	var candidates []string

	// Check for cmd/*/ directories with main.go
	entries, err := os.ReadDir("cmd")
	if err == nil {
		for _, e := range entries {
			if e.IsDir() {
				mainPath := fmt.Sprintf("cmd/%s/main.go", e.Name())
				if _, err := os.Stat(mainPath); err == nil {
					candidates = append(candidates, "./cmd/"+e.Name())
				}
			}
		}
	}

	// Check for main.go in root
	if _, err := os.Stat("main.go"); err == nil {
		candidates = append(candidates, "./")
	}

	if len(candidates) == 0 {
		return "", errors.New("no main package found. Create cmd/<app>/main.go or main.go in root")
	}

	if len(candidates) == 1 {
		return candidates[0], nil
	}

	fmt.Println("\nFound multiple main packages:")
	for i, c := range candidates {
		fmt.Printf("  %d) %s\n", i+1, c)
	}

	choice := promptInt("Select package to deploy", 1, len(candidates))
	return candidates[choice-1], nil
}

func selectOrCreateService(project, region string) (string, error) {
	// Check if already set in environment
	if service := os.Getenv("CLOUD_RUN_SERVICE"); service != "" {
		return service, nil
	}

	// List existing services
	fmt.Println("Fetching existing Cloud Run services...")
	services, err := gcloud.ListServices(project, region)
	if err != nil || len(services) == 0 {
		if err != nil {
			fmt.Println("No existing services found or API not enabled.")
		}
		return createServiceName()
	}

	fmt.Println("\nExisting services:")
	for i, s := range services {
		fmt.Printf("  %d) %s\n", i+1, s.Name)
	}
	fmt.Printf("  %d) Create new service\n", len(services)+1)

	choice := promptInt("Select service", 1, len(services)+1)
	if choice == len(services)+1 {
		return createServiceName()
	}

	return services[choice-1].Name, nil
}

func createServiceName() (string, error) {
	// Try to get a sensible default from go.mod
	defaultName := ""
	if data, err := os.ReadFile("go.mod"); err == nil {
		lines := strings.Split(string(data), "\n")
		if len(lines) > 0 && strings.HasPrefix(lines[0], "module ") {
			parts := strings.Split(strings.TrimPrefix(lines[0], "module "), "/")
			defaultName = parts[len(parts)-1]
		}
	}

	var promptMsg string
	if defaultName != "" {
		promptMsg = fmt.Sprintf("Enter service name [%s]", defaultName)
	} else {
		promptMsg = "Enter service name"
	}

	name := prompt(promptMsg)
	if name == "" {
		if defaultName != "" {
			return defaultName, nil
		}
		return "", errors.New("service name cannot be empty")
	}
	return name, nil
}

func saveDeploySettings(project, region, service, buildPath string) error {
	entries := []string{
		fmt.Sprintf("export CLOUDSDK_CORE_PROJECT=%s", project),
		fmt.Sprintf("export CLOUDSDK_RUN_REGION=%s", region),
		fmt.Sprintf("export CLOUD_RUN_SERVICE=%s", service),
		fmt.Sprintf("export KO_DOCKER_REPO=gcr.io/%s/%s", project, service),
		fmt.Sprintf("export KO_BUILD_PATH=%s", buildPath),
	}

	existing := make(map[string]bool)
	existingKeys := make(map[string]string) // key -> full line

	if file, err := os.Open(".envrc"); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			existing[line] = true
			// Extract key from "export KEY=value"
			if strings.HasPrefix(line, "export ") {
				parts := strings.SplitN(strings.TrimPrefix(line, "export "), "=", 2)
				if len(parts) == 2 {
					existingKeys[parts[0]] = line
				}
			}
		}
		_ = file.Close()
	}

	var toAdd []string
	var toUpdate []struct{ old, new string }

	for _, entry := range entries {
		if existing[entry] {
			continue
		}
		// Check if key exists with different value
		parts := strings.SplitN(strings.TrimPrefix(entry, "export "), "=", 2)
		if len(parts) == 2 {
			if oldLine, found := existingKeys[parts[0]]; found {
				toUpdate = append(toUpdate, struct{ old, new string }{oldLine, entry})
				continue
			}
		}
		toAdd = append(toAdd, entry)
	}

	// Update existing entries
	if len(toUpdate) > 0 {
		data, err := os.ReadFile(".envrc")
		if err != nil {
			return errors.WithStack(err)
		}
		content := string(data)
		for _, u := range toUpdate {
			content = strings.Replace(content, u.old, u.new, 1)
		}
		if err := os.WriteFile(".envrc", []byte(content), 0644); err != nil {
			return errors.WithStack(err)
		}
		for _, u := range toUpdate {
			fmt.Printf("Updated .envrc: %s\n", u.new)
		}
	}

	// Add new entries
	if len(toAdd) > 0 {
		file, err := os.OpenFile(".envrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return errors.WithStack(err)
		}
		defer func() { _ = file.Close() }()

		for _, entry := range toAdd {
			if _, err := file.WriteString(entry + "\n"); err != nil {
				return errors.WithStack(err)
			}
			fmt.Printf("Added to .envrc: %s\n", entry)
		}
	}

	// Remind user to run direnv allow
	if len(toAdd) > 0 || len(toUpdate) > 0 {
		fmt.Println("\nRun 'direnv allow' to load the new environment variables.")
	}

	return nil
}

func deployWithKo(project, region, service, buildPath, tag string) error {
	// Enable required APIs if not already enabled
	if err := gcloud.EnsureAPIs(project, "run.googleapis.com", "artifactregistry.googleapis.com"); err != nil {
		return err
	}

	// Configure docker auth for GCR if not already configured
	if err := gcloud.EnsureDockerAuth(); err != nil {
		return err
	}

	// Set KO_DOCKER_REPO for ko
	koRepo := fmt.Sprintf("gcr.io/%s/%s", project, service)
	if err := os.Setenv("KO_DOCKER_REPO", koRepo); err != nil {
		return errors.WithStack(err)
	}

	// Build and push with ko
	fmt.Println("\nBuilding and pushing image with ko...")
	fmt.Printf(" → ko build %s --bare\n", buildPath)

	var imageOut bytes.Buffer
	koCmd := exec.Command("ko", "build", buildPath, "--bare")
	koCmd.Stdout = &imageOut
	koCmd.Stderr = os.Stderr

	if err := koCmd.Run(); err != nil {
		return errors.Wrap(err, "ko build failed")
	}

	image := strings.TrimSpace(imageOut.String())
	if image == "" {
		return errors.New("ko build did not return image reference")
	}
	fmt.Printf("Built image: %s\n", image)

	// Deploy to Cloud Run
	if tag != "" {
		fmt.Printf("\nDeploying to Cloud Run service '%s' with tag '%s'...\n", service, tag)
		if err := gcloud.DeployWithTag(project, region, service, image, tag); err != nil {
			return err
		}

		// Get the tagged URL
		if url := gcloud.TagURL(project, region, service, tag); url != "" {
			fmt.Printf("\nTagged deploy successful!\nURL: %s\n", url)
		}
	} else {
		fmt.Printf("\nDeploying to Cloud Run service '%s'...\n", service)
		if err := gcloud.Deploy(project, region, service, image); err != nil {
			return err
		}

		// Get the service URL
		if url := gcloud.ServiceURL(project, region, service); url != "" {
			fmt.Printf("\nService deployed successfully!\nURL: %s\n", url)
		}
	}

	return nil
}

func prompt(msg string) string {
	fmt.Printf("%s: ", msg)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(line)
}

func promptInt(msg string, min, max int) int {
	for {
		fmt.Printf("%s (%d-%d): ", msg, min, max)
		reader := bufio.NewReader(os.Stdin)
		line, _ := reader.ReadString('\n')
		line = strings.TrimSpace(line)

		var n int
		if _, err := fmt.Sscanf(line, "%d", &n); err != nil || n < min || n > max {
			fmt.Printf("Please enter a number between %d and %d\n", min, max)
			continue
		}
		return n
	}
}

func confirm(msg string) bool {
	fmt.Printf("%s (y/n): ", msg)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.ToLower(strings.TrimSpace(line))
	return line == "y" || line == "yes"
}

func deleteTrafficTag(tag string) error {
	project := os.Getenv("CLOUDSDK_CORE_PROJECT")
	region := os.Getenv("CLOUDSDK_RUN_REGION")
	service := os.Getenv("CLOUD_RUN_SERVICE")

	if project == "" || region == "" || service == "" {
		return errors.New("no service deployed. Run 'go do deploy' first")
	}

	fmt.Printf("Removing tag '%s' from service '%s'...\n", tag, service)
	return gcloud.RemoveTag(project, region, service, tag)
}

func init() {
	deployCmd.Flags().StringVarP(&deployTag, "tag", "t", "", "deploy with a traffic tag (for branch deploys)")
	deployCmd.Flags().StringVar(&deleteTag, "delete-tag", "", "remove a traffic tag")
	rootCmd.AddCommand(deployCmd)
}
