//go:build dev

package wiring

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"

	"github.com/pocket/hexagon/spire/internal/controlplane/adapters/helm"
)

// BootstrapMinikubeInfra sets up the Minikube dev infrastructure
// This is dev-only code that won't be included in production builds
func BootstrapMinikubeInfra(ctx context.Context) error {
	fmt.Println("=== Bootstrapping Minikube Dev Infrastructure ===")

	// Get project root
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filename))
	infraDir := filepath.Join(projectRoot, "infra", "dev", "minikube")

	fmt.Printf("→ Project root: %s\n", projectRoot)
	fmt.Printf("→ Infra dir: %s\n", infraDir)

	// Create Helm installer
	installer, err := helm.NewInstaller(infraDir)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Enable debug mode
	installer.SetDebug(true)

	// Apply configuration
	namespace := "spire-system"
	valuesFile := "values-minikube.yaml"

	if err := installer.Apply(ctx, namespace, valuesFile); err != nil {
		return fmt.Errorf("failed to apply configuration: %w", err)
	}

	fmt.Println("✓ Infrastructure bootstrap complete")
	return nil
}

// DestroyMinikubeInfra tears down the Minikube dev infrastructure
func DestroyMinikubeInfra(ctx context.Context) error {
	fmt.Println("=== Destroying Minikube Dev Infrastructure ===")

	// Get project root
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filename))
	infraDir := filepath.Join(projectRoot, "infra", "dev", "minikube")

	// Create Helm installer
	installer, err := helm.NewInstaller(infraDir)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Destroy
	namespace := "spire-system"
	if err := installer.Destroy(ctx, namespace); err != nil {
		return fmt.Errorf("failed to destroy infrastructure: %w", err)
	}

	fmt.Println("✓ Infrastructure destroyed")
	return nil
}

// GetMinikubeInfraStatus returns the status of the Minikube infrastructure
func GetMinikubeInfraStatus(ctx context.Context) error {
	fmt.Println("=== Minikube Infrastructure Status ===")

	// Get project root
	_, filename, _, _ := runtime.Caller(0)
	projectRoot := filepath.Dir(filepath.Dir(filename))
	infraDir := filepath.Join(projectRoot, "infra", "dev", "minikube")

	// Create Helm installer
	installer, err := helm.NewInstaller(infraDir)
	if err != nil {
		return fmt.Errorf("failed to create installer: %w", err)
	}

	// Get status
	namespace := "spire-system"
	if err := installer.Status(ctx, namespace); err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	return nil
}
