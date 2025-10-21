//go:build dev

package helm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Apply deploys charts using helmfile or helm
func (i *Installer) Apply(ctx context.Context, namespace, valuesFile string) error {
	if i.helmfilePath != "" {
		return i.applyWithHelmfile(ctx, namespace)
	}
	return i.applyWithHelm(ctx, namespace, valuesFile)
}

// applyWithHelmfile uses helmfile for deployment
func (i *Installer) applyWithHelmfile(ctx context.Context, namespace string) error {
	fmt.Printf("→ Deploying with helmfile (namespace: %s)\n", namespace)

	args := []string{
		"-e", "dev",
		"apply",
	}

	if i.dryRun {
		args = append(args, "--dry-run")
	}

	if i.debug {
		args = append(args, "--debug")
	}

	cmd := exec.CommandContext(ctx, i.helmfilePath, args...)
	cmd.Dir = i.infraDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helmfile apply failed: %w", err)
	}

	fmt.Println("✓ Helmfile apply completed")
	return nil
}

// applyWithHelm uses direct helm commands
func (i *Installer) applyWithHelm(ctx context.Context, namespace, valuesFile string) error {
	fmt.Printf("→ Deploying with helm (namespace: %s)\n", namespace)

	// Create namespace
	if err := i.createNamespace(ctx, namespace); err != nil {
		return err
	}

	// Add SPIRE repository
	if err := i.addRepo(ctx, "spiffe", "https://spiffe.github.io/helm-charts-hardened/"); err != nil {
		return err
	}

	// Update repos
	if err := i.updateRepos(ctx); err != nil {
		return err
	}

	valuesPath := filepath.Join(i.infraDir, valuesFile)

	// Install SPIRE server
	if err := i.installChart(ctx, "spire-server", "spiffe/spire-server", namespace, valuesPath); err != nil {
		return err
	}

	// Install SPIRE agent
	if err := i.installChart(ctx, "spire-agent", "spiffe/spire-agent", namespace, valuesPath); err != nil {
		return err
	}

	fmt.Println("✓ Helm install completed")
	return nil
}

// Destroy removes deployed charts
func (i *Installer) Destroy(ctx context.Context, namespace string) error {
	if i.helmfilePath != "" {
		return i.destroyWithHelmfile(ctx)
	}
	return i.destroyWithHelm(ctx, namespace)
}

// destroyWithHelmfile uses helmfile for removal
func (i *Installer) destroyWithHelmfile(ctx context.Context) error {
	fmt.Println("→ Destroying with helmfile")

	cmd := exec.CommandContext(ctx, i.helmfilePath, "-e", "dev", "destroy")
	cmd.Dir = i.infraDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helmfile destroy failed: %w", err)
	}

	fmt.Println("✓ Helmfile destroy completed")
	return nil
}

// destroyWithHelm uses direct helm commands
func (i *Installer) destroyWithHelm(ctx context.Context, namespace string) error {
	fmt.Println("→ Destroying with helm")

	// Uninstall agent first
	_ = i.uninstallChart(ctx, "spire-agent", namespace)

	// Then server
	_ = i.uninstallChart(ctx, "spire-server", namespace)

	fmt.Println("✓ Helm uninstall completed")
	return nil
}

// Status returns the status of deployed releases
func (i *Installer) Status(ctx context.Context, namespace string) error {
	fmt.Printf("→ Checking status in namespace %s\n", namespace)

	cmd := exec.CommandContext(ctx, i.helmPath,
		"list", "-n", namespace)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to list releases: %w", err)
	}

	return nil
}
