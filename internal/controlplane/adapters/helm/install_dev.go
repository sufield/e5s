//go:build dev

package helm

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Installer provides Helm installation functionality for dev environments
type Installer struct {
	helmPath     string
	helmfilePath string
	kubectlPath  string
	infraDir     string
	timeout      time.Duration
	dryRun       bool
	debug        bool
}

// NewInstaller creates a new Helm installer
func NewInstaller(infraDir string) (*Installer, error) {
	// Find helm binary
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return nil, fmt.Errorf("helm not found in PATH: %w", err)
	}

	// Find helmfile (optional)
	helmfilePath, _ := exec.LookPath("helmfile")

	// Find kubectl
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("kubectl not found in PATH: %w", err)
	}

	return &Installer{
		helmPath:     helmPath,
		helmfilePath: helmfilePath,
		kubectlPath:  kubectlPath,
		infraDir:     infraDir,
		timeout:      5 * time.Minute,
		dryRun:       false,
		debug:        false,
	}, nil
}

// SetTimeout sets the operation timeout
func (i *Installer) SetTimeout(timeout time.Duration) {
	i.timeout = timeout
}

// SetDryRun enables dry-run mode
func (i *Installer) SetDryRun(dryRun bool) {
	i.dryRun = dryRun
}

// SetDebug enables debug output
func (i *Installer) SetDebug(debug bool) {
	i.debug = debug
}

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

// Helper methods

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

func (i *Installer) addRepo(ctx context.Context, name, url string) error {
	fmt.Printf("→ Adding Helm repo %s\n", name)

	cmd := exec.CommandContext(ctx, i.helmPath, "repo", "add", name, url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Ignore error if repo already exists
	_ = cmd.Run()

	return nil
}

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
