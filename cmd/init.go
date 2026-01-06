package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

var allow bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize an app for `go do`",
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := exec.LookPath("direnv"); err != nil {
			return errors.New("direnv is not installed")
		}

		if err := updateEnvrc(); err != nil {
			return err
		}

		if err := os.MkdirAll(".claude", 0755); err != nil {
			return errors.WithStack(err)
		}
		if err := updateClaudeSettings(); err != nil {
			return err
		}

		if err := updateGitignore(); err != nil {
			return err
		}

		if err := writeGoWrapper(); err != nil {
			return err
		}

		if allow {
			cmd := exec.Command("direnv", "allow")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	},
}

func updateEnvrc() error {
	entries := []string{"export GO=$(which go)", "PATH_add bin"}
	existing := make(map[string]bool)

	if file, err := os.Open(".envrc"); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			existing[strings.TrimSpace(scanner.Text())] = true
		}
		_ = file.Close()
	}

	var toAdd []string
	for _, entry := range entries {
		if !existing[entry] {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	file, err := os.OpenFile(".envrc", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = file.Close() }()

	for _, entry := range toAdd {
		if _, err := file.WriteString(entry + "\n"); err != nil {
			return errors.WithStack(err)
		}
	}

	fmt.Printf("Updated .envrc with: %s\n", strings.Join(toAdd, ", "))
	return nil
}

func updateClaudeSettings() error {
	const name = ".claude/settings.json"
	perms := []string{
		"Bash(go:*)",
		"Bash(git:*)",
		"Bash(gh:*)",
		"Bash(ls:*)",
		"Bash(tree:*)",
		"Bash(cat:*)",
		"Bash(find:*)",
		"Bash(grep:*)",
		"Bash(mkdir:*)",
		"Bash(mv:*)",
		"Bash(sed:*)",
		"Bash(awk:*)",
		"Bash(xargs:*)",
		"Bash(wc:*)",
		"Bash(jq:*)",
		"Bash(curl:*)",
		"Bash(psql:*)",
		"Bash(sqlite:*)",
		"Bash(sqlite3:*)",
		"Bash(sqlc:*)",
		"Bash(templ:*)",
	}

	var settings map[string]any

	data, err := os.ReadFile(name)
	if err == nil {
		if err := json.Unmarshal(data, &settings); err != nil {
			return errors.WithStack(err)
		}
	} else {
		settings = make(map[string]any)
	}

	permissions, ok := settings["permissions"].(map[string]any)
	if !ok {
		permissions = make(map[string]any)
		settings["permissions"] = permissions
	}

	allow, ok := permissions["allow"].([]any)
	if !ok {
		allow = []any{}
	}

	existing := make(map[string]bool)
	for _, p := range allow {
		if s, ok := p.(string); ok {
			existing[s] = true
		}
	}

	var added []string
	for _, perm := range perms {
		if !existing[perm] {
			allow = append(allow, perm)
			added = append(added, perm)
		}
	}

	if len(added) == 0 {
		return nil
	}

	permissions["allow"] = allow

	out, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return errors.WithStack(err)
	}

	if err := os.WriteFile(name, append(out, '\n'), 0644); err != nil {
		return errors.WithStack(err)
	}

	fmt.Printf("Updated %s with permissions: %s\n", name, strings.Join(added, ", "))
	return nil
}

func updateGitignore() error {
	entries := []string{".claude", ".envrc", "bin/do"}
	existing := make(map[string]bool)

	if file, err := os.Open(".gitignore"); err == nil {
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			existing[strings.TrimSpace(scanner.Text())] = true
		}
		_ = file.Close()
	}

	var toAdd []string
	for _, entry := range entries {
		if !existing[entry] {
			toAdd = append(toAdd, entry)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	file, err := os.OpenFile(".gitignore", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.WithStack(err)
	}
	defer func() { _ = file.Close() }()

	for _, entry := range toAdd {
		if _, err := file.WriteString(entry + "\n"); err != nil {
			return errors.WithStack(err)
		}
	}

	fmt.Printf("Updated .gitignore with: %s\n", strings.Join(toAdd, ", "))
	return nil
}

func writeGoWrapper() error {
	const script = `#!/bin/bash
set -e
case "$1" in
  do) shift; exec "$GO" tool do "$@" ;;
  *)  exec "$GO" "$@" ;;
esac
`
	if err := os.MkdirAll("bin", 0755); err != nil {
		return errors.WithStack(err)
	}

	if err := os.WriteFile("bin/go", []byte(script), 0755); err != nil {
		return errors.WithStack(err)
	}

	fmt.Println("Created bin/go")
	return nil
}

func init() {
	initCmd.Flags().BoolVarP(&allow, "allow", "a", false, "automatically run direnv allow")
	rootCmd.AddCommand(initCmd)
}
