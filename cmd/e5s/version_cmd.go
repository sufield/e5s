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
	verbose := fs.Bool("verbose", false, "Show detailed version information")
	format := fs.String("format", "table", "Output format: table, plain")

	fs.Usage = func() {
		fmt.Println(`Show version information for e5s and dependencies

USAGE:
    e5s version [flags]

FLAGS:
    --format string    Output format: table, plain (default "table")
    --verbose          Show detailed version information including TLS settings

FORMATS:
    table      Human-readable tables with borders (default)
    plain      Plain text output for piping to other commands

EXAMPLES:
    # Show e5s version
    e5s version

    # Show detailed information including TLS config
    e5s version --verbose

    # Output in plain format for piping
    e5s version --format plain

    # Extract specific version
    e5s version --format plain | grep go=`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	return showRuntimeVersions(*verbose, *format)
}

func showRuntimeVersions(verbose bool, format string) error {
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

	// Collect tool versions
	toolVersions := make([]struct {
		name    string
		version string
		status  string
	}, len(tools))

	for i, tool := range tools {
		ver, err := getToolVersion(tool.command)
		status := "✓"
		if err != nil {
			if tool.required {
				status = "✗ REQUIRED"
			} else {
				status = "○ Optional"
			}
			ver = "not found"
		}
		toolVersions[i] = struct {
			name    string
			version string
			status  string
		}{tool.name, ver, status}
	}

	// Output based on format
	if format == "plain" {
		fmt.Printf("e5s_version=%s\n", version)
		fmt.Printf("e5s_commit=%s\n", commit)
		fmt.Printf("e5s_built=%s\n", date)
		for _, tv := range toolVersions {
			fmt.Printf("%s=%s\n", strings.ToLower(tv.name), tv.version)
		}
		return nil
	}

	// Table format (default)
	fmt.Printf("e5s CLI %s (commit: %s, built: %s)\n\n", version, commit, date)

	fmt.Println("TLS Configuration:")
	printTLSConfig()
	fmt.Println()

	fmt.Println("Runtime Environment:")
	table := NewTableWriter([]string{"Component", "Version", "Status"})
	for _, tv := range toolVersions {
		table.AddRow([]string{tv.name, tv.version, tv.status})
	}
	table.Print()

	if verbose {
		fmt.Println("\nDetailed Information:")
		showDetailedInfo()
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
