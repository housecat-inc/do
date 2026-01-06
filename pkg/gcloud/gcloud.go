package gcloud

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
)

// Project represents a GCP project.
type Project struct {
	ID   string
	Name string
}

// Service represents a Cloud Run service.
type Service struct {
	Name string
}

// IsInstalled checks if gcloud CLI is installed.
func IsInstalled() bool {
	_, err := exec.LookPath("gcloud")
	return err == nil
}

// IsAuthenticated checks if gcloud is authenticated.
func IsAuthenticated() bool {
	cmd := exec.Command("gcloud", "auth", "print-access-token")
	return cmd.Run() == nil
}

// Login starts the gcloud login flow.
func Login() error {
	return Run("gcloud", "auth", "login")
}

// CurrentProject returns the currently configured project, if any.
func CurrentProject() string {
	cmd := exec.Command("gcloud", "config", "get-value", "project")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	project := strings.TrimSpace(string(out))
	if project == "(unset)" {
		return ""
	}
	return project
}

// ListProjects returns all accessible GCP projects.
func ListProjects() ([]Project, error) {
	cmd := exec.Command("gcloud", "projects", "list", "--format=json")
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrap(err, "failed to list projects")
	}

	var raw []struct {
		ProjectID string `json:"projectId"`
		Name      string `json:"name"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, errors.Wrap(err, "failed to parse projects")
	}

	projects := make([]Project, len(raw))
	for i, p := range raw {
		projects[i] = Project{ID: p.ProjectID, Name: p.Name}
	}
	return projects, nil
}

// CreateProject creates a new GCP project.
func CreateProject(projectID string) error {
	return Run("gcloud", "projects", "create", projectID)
}

// EnsureAPIs enables the specified APIs if not already enabled.
func EnsureAPIs(project string, apis ...string) error {
	cmd := exec.Command("gcloud", "services", "list", "--enabled", "--format=value(config.name)", "--project", project)
	out, err := cmd.Output()
	if err != nil {
		// Can't check, just try to enable all
		args := append([]string{"services", "enable"}, apis...)
		args = append(args, "--project", project)
		return Run("gcloud", args...)
	}

	enabled := make(map[string]bool)
	for _, line := range strings.Split(string(out), "\n") {
		enabled[strings.TrimSpace(line)] = true
	}

	var toEnable []string
	for _, api := range apis {
		if !enabled[api] {
			toEnable = append(toEnable, api)
		}
	}

	if len(toEnable) == 0 {
		return nil
	}

	fmt.Printf("Enabling APIs: %s\n", strings.Join(toEnable, ", "))
	args := append([]string{"services", "enable"}, toEnable...)
	args = append(args, "--project", project)
	return Run("gcloud", args...)
}

// EnsureDockerAuth configures docker authentication for gcr.io.
func EnsureDockerAuth() error {
	home, err := os.UserHomeDir()
	if err == nil {
		data, err := os.ReadFile(home + "/.docker/config.json")
		if err == nil && strings.Contains(string(data), "gcr.io") {
			return nil
		}
	}

	fmt.Println("Configuring Docker authentication for gcr.io...")
	return Run("gcloud", "auth", "configure-docker", "gcr.io", "--quiet")
}

// ListServices returns Cloud Run services in the specified project/region.
func ListServices(project, region string) ([]Service, error) {
	cmd := exec.Command("gcloud", "run", "services", "list",
		"--platform=managed",
		"--region="+region,
		"--project="+project,
		"--format=json")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var raw []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(out, &raw); err != nil {
		return nil, err
	}

	services := make([]Service, len(raw))
	for i, s := range raw {
		services[i] = Service{Name: s.Metadata.Name}
	}
	return services, nil
}

// Deploy deploys an image to Cloud Run and routes 100% traffic to it.
func Deploy(project, region, service, image string) error {
	if err := Run("gcloud", "run", "deploy", service,
		"--image="+image,
		"--platform=managed",
		"--region="+region,
		"--project="+project,
		"--allow-unauthenticated"); err != nil {
		return err
	}

	// Ensure 100% traffic goes to latest revision
	return Run("gcloud", "run", "services", "update-traffic", service,
		"--platform=managed",
		"--region="+region,
		"--project="+project,
		"--to-latest")
}

// DeployWithTag deploys an image with a traffic tag (for branch deploys).
// The tag gets its own URL without receiving production traffic.
func DeployWithTag(project, region, service, image, tag string) error {
	return Run("gcloud", "run", "deploy", service,
		"--image="+image,
		"--platform=managed",
		"--region="+region,
		"--project="+project,
		"--tag="+tag,
		"--no-traffic",
		"--allow-unauthenticated")
}

// ServiceURL returns the URL of a Cloud Run service.
func ServiceURL(project, region, service string) string {
	cmd := exec.Command("gcloud", "run", "services", "describe", service,
		"--platform=managed",
		"--region="+region,
		"--project="+project,
		"--format=value(status.url)")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// RemoveTag removes a traffic tag from a Cloud Run service.
func RemoveTag(project, region, service, tag string) error {
	return Run("gcloud", "run", "services", "update-traffic", service,
		"--platform=managed",
		"--region="+region,
		"--project="+project,
		"--remove-tags="+tag)
}

// TagURL returns the URL for a specific traffic tag.
func TagURL(project, region, service, tag string) string {
	cmd := exec.Command("gcloud", "run", "services", "describe", service,
		"--platform=managed",
		"--region="+region,
		"--project="+project,
		"--format=json(status.traffic)")
	out, err := cmd.Output()
	if err != nil {
		return ""
	}

	var result struct {
		Status struct {
			Traffic []struct {
				Tag string `json:"tag"`
				URL string `json:"url"`
			} `json:"traffic"`
		} `json:"status"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return ""
	}

	for _, t := range result.Status.Traffic {
		if t.Tag == tag {
			return t.URL
		}
	}
	return ""
}

// Run executes a gcloud command with output to stdout/stderr.
func Run(name string, args ...string) error {
	fmt.Printf(" → %s %s\n", name, strings.Join(args, " "))
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.WithStack(err)
	}
	return nil
}
