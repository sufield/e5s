//go:build dev

package helm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// createNamespace creates a Kubernetes namespace if it doesn't exist
func (i *Installer) createNamespace(ctx context.Context, namespace string) error {
	fmt.Printf("→ Creating namespace %s\n", namespace)

	cmd := exec.CommandContext(ctx, i.kubectlPath,
		"create", "namespace", namespace,
		"--dry-run=client", "-o", "yaml")

	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to generate namespace yaml: %w", err)
	}

	applyCmd := exec.CommandContext(ctx, i.kubectlPath, "apply", "-f", "-")
	applyCmd.Stdin = strings.NewReader(string(output))
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr

	if err := applyCmd.Run(); err != nil {
		return fmt.Errorf("failed to create namespace: %w", err)
	}

	return nil
}

// addRepo adds a Helm repository
func (i *Installer) addRepo(ctx context.Context, name, url string) error {
	fmt.Printf("→ Adding Helm repo %s\n", name)

	cmd := exec.CommandContext(ctx, i.helmPath, "repo", "add", name, url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Ignore error if repo already exists
	_ = cmd.Run()

	return nil
}

// updateRepos updates all Helm repositories
func (i *Installer) updateRepos(ctx context.Context) error {
	fmt.Println("→ Updating Helm repos")

	cmd := exec.CommandContext(ctx, i.helmPath, "repo", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update repos: %w", err)
	}

	return nil
}

// installChart installs or upgrades a Helm chart
func (i *Installer) installChart(ctx context.Context, releaseName, chart, namespace, valuesFile string) error {
	fmt.Printf("→ Installing %s\n", releaseName)

	args := []string{
		"upgrade", "--install",
		releaseName, chart,
		"-n", namespace,
		"-f", valuesFile,
		"--wait",
		"--timeout", i.timeout.String(),
	}

	if i.dryRun {
		args = append(args, "--dry-run")
	}

	if i.debug {
		args = append(args, "--debug")
	}

	cmd := exec.CommandContext(ctx, i.helmPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install %s: %w", releaseName, err)
	}

	return nil
}

// uninstallChart uninstalls a Helm chart
func (i *Installer) uninstallChart(ctx context.Context, releaseName, namespace string) error {
	fmt.Printf("→ Uninstalling %s\n", releaseName)

	cmd := exec.CommandContext(ctx, i.helmPath,
		"uninstall", releaseName,
		"-n", namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Ignore errors (chart may not exist)
	_ = cmd.Run()

	return nil
}
