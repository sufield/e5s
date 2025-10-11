//go:build dev

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/pocket/hexagon/spire/wiring"
)

const (
	cmdUp     = "up"
	cmdDown   = "down"
	cmdStatus = "status"
)

func main() {
	// Parse flags
	var (
		timeout = flag.Duration("timeout", 5*time.Minute, "Operation timeout")
		debug   = flag.Bool("debug", false, "Enable debug output")
	)
	flag.Parse()

	// Get command
	args := flag.Args()
	if len(args) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := args[0]

	// Setup context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	// Handle interrupt
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\n→ Interrupted, cancelling...")
		cancel()
	}()

	// Enable debug if requested
	if *debug {
		fmt.Println("→ Debug mode enabled")
	}

	// Execute command
	var err error
	switch command {
	case cmdUp:
		err = runUp(ctx)
	case cmdDown:
		err = runDown(ctx)
	case cmdStatus:
		err = runStatus(ctx)
	default:
		fmt.Printf("Error: Unknown command '%s'\n\n", command)
		printUsage()
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✓ Command completed successfully")
}

func runUp(ctx context.Context) error {
	fmt.Println("=== Starting Minikube Infrastructure ===")

	if err := wiring.BootstrapMinikubeInfra(ctx); err != nil {
		return fmt.Errorf("failed to bootstrap infrastructure: %w", err)
	}

	// Display info
	fmt.Println("\n=== Infrastructure Ready ===")
	fmt.Println("SPIRE is now running on Minikube")
	fmt.Println("\nUseful commands:")
	fmt.Println("  kubectl get pods -n spire-system")
	fmt.Println("  kubectl logs -n spire-system deployment/spire-server")
	fmt.Println("  kubectl logs -n spire-system daemonset/spire-agent")
	fmt.Println("  minikube dashboard")

	return nil
}

func runDown(ctx context.Context) error {
	fmt.Println("=== Stopping Minikube Infrastructure ===")

	if err := wiring.DestroyMinikubeInfra(ctx); err != nil {
		return fmt.Errorf("failed to destroy infrastructure: %w", err)
	}

	fmt.Println("\n=== Infrastructure Stopped ===")
	fmt.Println("SPIRE has been removed from Minikube")

	return nil
}

func runStatus(ctx context.Context) error {
	if err := wiring.GetMinikubeInfraStatus(ctx); err != nil {
		return fmt.Errorf("failed to get status: %w", err)
	}

	return nil
}

func printUsage() {
	fmt.Println("cp-minikube - Minikube Control Plane Manager (Dev Only)")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  cp-minikube [flags] <command>")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  up      Start Minikube infrastructure and deploy SPIRE")
	fmt.Println("  down    Stop and remove SPIRE from Minikube")
	fmt.Println("  status  Show current infrastructure status")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -timeout duration")
	fmt.Println("        Operation timeout (default 5m)")
	fmt.Println("  -debug")
	fmt.Println("        Enable debug output")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Start infrastructure")
	fmt.Println("  cp-minikube up")
	fmt.Println()
	fmt.Println("  # Stop infrastructure")
	fmt.Println("  cp-minikube down")
	fmt.Println()
	fmt.Println("  # Check status")
	fmt.Println("  cp-minikube status")
	fmt.Println()
	fmt.Println("  # Start with custom timeout")
	fmt.Println("  cp-minikube -timeout 10m up")
	fmt.Println()
	fmt.Println("Note: This binary is only built with -tags=dev")
}
