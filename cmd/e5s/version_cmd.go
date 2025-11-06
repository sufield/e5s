package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func versionCommand(args []string) error {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	mode := fs.String("mode", "runtime", "Mode: runtime, dev, prod")
	verbose := fs.Bool("verbose", false, "Show detailed version information")

	fs.Usage = func() {
		fmt.Println(`Show version information for e5s and dependencies

USAGE:
    e5s version [flags]

FLAGS:
    --mode string      Environment mode: runtime, dev, prod (default "runtime")
    --verbose          Show detailed version information including TLS settings

MODES:
    runtime    Show currently installed versions (default)
    dev        Show required versions for development
    prod       Show required versions for production deployment

EXAMPLES:
    # Show e5s version
    e5s version

    # Show all runtime versions
    e5s version --mode runtime

    # Show development requirements
    e5s version --mode dev

    # Show production requirements
    e5s version --mode prod

    # Show detailed information including TLS config
    e5s version --verbose`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	switch *mode {
	case "runtime":
		return showRuntimeVersions(*verbose)
	case "dev":
		return showDevRequirements(*verbose)
	case "prod":
		return showProdRequirements(*verbose)
	default:
		return fmt.Errorf("unknown mode: %s (use 'runtime', 'dev', or 'prod')", *mode)
	}
}

func showRuntimeVersions(verbose bool) error {
	fmt.Printf("e5s CLI %s (commit: %s, built: %s)\n\n", version, commit, date)

	// TLS Configuration
	fmt.Println("TLS Configuration:")
	printTLSConfig()
	fmt.Println()

	// Detect and show runtime versions
	fmt.Println("Runtime Environment:")
	table := NewTableWriter([]string{"Component", "Version", "Status"})

	tools := []struct {
		name     string
		command  []string
		required bool
	}{
		{"Go", []string{"go", "version"}, true},
		{"Docker", []string{"docker", "version", "--format", "{{.Client.Version}}"}, false},
		{"kubectl", []string{"kubectl", "version", "--client", "--short"}, false},
		{"Helm", []string{"helm", "version", "--short"}, false},
		{"Minikube", []string{"minikube", "version", "--short"}, false},
	}

	for _, tool := range tools {
		version, err := getToolVersion(tool.command)
		status := "✓"
		if err != nil {
			if tool.required {
				status = "✗ REQUIRED"
			} else {
				status = "○ Optional"
			}
			version = "not found"
		}
		table.AddRow([]string{tool.name, version, status})
	}

	table.Print()

	if verbose {
		fmt.Println("\nDetailed Information:")
		showDetailedInfo()
	}

	return nil
}

func showDevRequirements(verbose bool) error {
	fmt.Printf("e5s Development Requirements\n\n")

	fmt.Println("Required Tools:")
	table := NewTableWriter([]string{"Tool", "Minimum Version", "Purpose"})
	table.AddRow([]string{"Go", "1.21+", "Language runtime"})
	table.AddRow([]string{"Docker", "20.10+", "Container images"})
	table.AddRow([]string{"Minikube", "1.30+", "Local Kubernetes"})
	table.AddRow([]string{"kubectl", "1.28+", "Kubernetes CLI"})
	table.AddRow([]string{"Helm", "3.12+", "SPIRE installation"})
	table.Print()

	fmt.Println("\nTLS Configuration:")
	printTLSConfig()

	if verbose {
		fmt.Println("\nDevelopment Workflow:")
		fmt.Println("  1. Install SPIRE via Helmfile (see SPIRE_SETUP.md)")
		fmt.Println("  2. Build test applications with local e5s code")
		fmt.Println("  3. Test locally with go run or build binaries")
		fmt.Println("  4. Deploy to Minikube for integration testing")
	}

	return nil
}

func showProdRequirements(verbose bool) error {
	fmt.Printf("e5s Production Requirements\n\n")

	fmt.Println("Required Components:")
	table := NewTableWriter([]string{"Component", "Minimum Version", "Purpose"})
	table.AddRow([]string{"Go", "1.21+", "Application runtime"})
	table.AddRow([]string{"SPIRE Server", "1.8+", "Identity provider"})
	table.AddRow([]string{"SPIRE Agent", "1.8+", "Workload API"})
	table.AddRow([]string{"Kubernetes", "1.28+", "Container orchestration"})
	table.AddRow([]string{"SPIRE CSI Driver", "0.2.6+", "Automatic registration"})
	table.Print()

	fmt.Println("\nTLS Configuration:")
	printTLSConfig()

	if verbose {
		fmt.Println("\nProduction Deployment:")
		fmt.Println("  1. Install SPIRE server in Kubernetes cluster")
		fmt.Println("  2. Install SPIRE CSI driver for automatic workload registration")
		fmt.Println("  3. Deploy e5s applications with proper ConfigMaps")
		fmt.Println("  4. Configure allowed_client_spiffe_id for zero-trust")
		fmt.Println("  5. Monitor SPIRE logs and certificate rotation")
		fmt.Println("\nSecurity Best Practices:")
		fmt.Println("  • Use specific SPIFFE IDs (not trust domain matching)")
		fmt.Println("  • Validate configurations with: e5s validate config.yaml")
		fmt.Println("  • Monitor certificate expiration and rotation")
		fmt.Println("  • Use network policies for defense in depth")
	}

	return nil
}

func printTLSConfig() {
	// Get TLS version names
	minVersion := getTLSVersionName(tls.VersionTLS13)

	table := NewTableWriter([]string{"Setting", "Value"})
	table.AddRow([]string{"Minimum TLS Version", minVersion})
	table.AddRow([]string{"Maximum TLS Version", "TLS 1.3"})
	table.AddRow([]string{"Client Auth", "Required (mTLS)"})
	table.AddRow([]string{"Certificate Source", "SPIRE Workload API"})
	table.AddRow([]string{"Certificate Rotation", "Automatic"})
	table.Print()
}

func getTLSVersionName(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLS 1.0"
	case tls.VersionTLS11:
		return "TLS 1.1"
	case tls.VersionTLS12:
		return "TLS 1.2"
	case tls.VersionTLS13:
		return "TLS 1.3"
	default:
		return fmt.Sprintf("Unknown (0x%04x)", version)
	}
}

func getToolVersion(command []string) (string, error) {
	// Validate command is in allowed list for security
	allowedCommands := map[string]bool{
		"go":       true,
		"docker":   true,
		"kubectl":  true,
		"helm":     true,
		"minikube": true,
	}

	if len(command) == 0 || !allowedCommands[command[0]] {
		return "", fmt.Errorf("unsupported command: %v", command)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, command[0], command[1:]...) // #nosec G204 - command validated against allowlist
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	// Clean up output
	result := strings.TrimSpace(string(output))

	// Extract version for different tools
	switch {
	case strings.Contains(command[0], "go"):
		// "go version go1.21.0 linux/amd64" -> "go1.21.0"
		parts := strings.Fields(result)
		if len(parts) >= 3 {
			return parts[2], nil
		}
	case strings.Contains(command[0], "kubectl"):
		// "Client Version: v1.28.0" -> "v1.28.0"
		result = strings.TrimPrefix(result, "Client Version: ")
	case strings.Contains(command[0], "minikube"):
		// "minikube version: v1.30.1" -> "v1.30.1"
		result = strings.TrimPrefix(result, "minikube version: ")
	case strings.Contains(command[0], "helm"):
		// "v3.12.0+g123abc" -> "v3.12.0"
		if idx := strings.Index(result, "+"); idx > 0 {
			result = result[:idx]
		}
	}

	return result, nil
}

func showDetailedInfo() {
	// Show Go environment
	fmt.Println("\nGo Environment:")
	if goroot, err := getToolVersion([]string{"go", "env", "GOROOT"}); err == nil {
		fmt.Printf("  GOROOT: %s\n", goroot)
	}
	if gopath, err := getToolVersion([]string{"go", "env", "GOPATH"}); err == nil {
		fmt.Printf("  GOPATH: %s\n", gopath)
	}

	// Show Docker info if available
	if _, err := getToolVersion([]string{"docker", "version"}); err == nil {
		if info, err := getToolVersion([]string{"docker", "info", "--format", "{{.ServerVersion}}"}); err == nil {
			fmt.Printf("\nDocker Server: %s\n", info)
		}
	}

	// Show Kubernetes context if available
	if ctx, err := getToolVersion([]string{"kubectl", "config", "current-context"}); err == nil {
		fmt.Printf("\nKubernetes Context: %s\n", ctx)
	}
}
