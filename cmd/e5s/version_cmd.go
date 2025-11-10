package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func versionCommand(args []string) error {
	fs := flag.NewFlagSet("version", flag.ExitOnError)
	format := fs.String("format", "json", "Output format: json, plain")
	mode := fs.String("mode", "", "Show requirements: dev, prod (default: show runtime versions)")

	fs.Usage = func() {
		fmt.Println(`Show version information for e5s and dependencies

USAGE:
    e5s version [flags]

FLAGS:
    --format string    Output format: json, plain (default "json")
    --mode string      Show requirements: dev, prod (default: show runtime versions)

MODES:
    (default)  Show actual runtime versions of installed tools
    dev        Show required tool versions for development
    prod       Show required components for production deployment

FORMATS:
    json       JSON output for automation and scripting (default)
    plain      Plain text output for simple parsing

EXAMPLES:
    # Show current runtime versions (JSON)
    e5s version

    # Show development requirements (JSON)
    e5s version --mode dev

    # Show production requirements (JSON)
    e5s version --mode prod

    # Output in plain format for simple parsing
    e5s version --format plain

    # Get dev requirements and query with jq
    e5s version --mode dev | jq '.requirements[] | select(.component=="Go")'

    # Extract Go version with jq
    e5s version | jq -r '.runtime[] | select(.component=="Go") | .version'

    # Extract specific version from plain format
    e5s version --format plain | grep go=`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	// Handle mode flag
	switch *mode {
	case "dev":
		return showDevRequirements(*format)
	case "prod":
		return showProdRequirements(*format)
	case "":
		return showRuntimeVersions(*format)
	default:
		return fmt.Errorf("invalid mode: %s (must be: dev, prod)", *mode)
	}
}

func showRuntimeVersions(format string) error {
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
	type toolVersion struct {
		Component string `json:"component"`
		Version   string `json:"version"`
		Status    string `json:"status"`
		Required  bool   `json:"required"`
	}

	toolVersions := make([]toolVersion, len(tools))

	for i, tool := range tools {
		ver, err := getToolVersion(tool.command)
		status := "installed"
		if err != nil {
			status = "not found"
			ver = "not found"
		}
		toolVersions[i] = toolVersion{
			Component: tool.name,
			Version:   ver,
			Status:    status,
			Required:  tool.required,
		}
	}

	// JSON output
	if format == "json" {
		type tlsConfig struct {
			MinTLSVersion       string `json:"min_tls_version"`
			MaxTLSVersion       string `json:"max_tls_version"`
			ClientAuth          string `json:"client_auth"`
			CertificateSource   string `json:"certificate_source"`
			CertificateRotation string `json:"certificate_rotation"`
		}

		type runtimeOutput struct {
			E5SVersion string        `json:"e5s_version"`
			E5SCommit  string        `json:"e5s_commit"`
			E5SBuilt   string        `json:"e5s_built"`
			TLSConfig  tlsConfig     `json:"tls_config"`
			Runtime    []toolVersion `json:"runtime"`
		}

		output := runtimeOutput{
			E5SVersion: version,
			E5SCommit:  commit,
			E5SBuilt:   date,
			TLSConfig: tlsConfig{
				MinTLSVersion:       "TLS 1.3",
				MaxTLSVersion:       "TLS 1.3",
				ClientAuth:          "Required (mTLS)",
				CertificateSource:   "SPIRE Workload API",
				CertificateRotation: "Automatic",
			},
			Runtime: toolVersions,
		}

		jsonBytes, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	// Plain text output
	if format == "plain" {
		fmt.Printf("e5s_version=%s\n", version)
		fmt.Printf("e5s_commit=%s\n", commit)
		fmt.Printf("e5s_built=%s\n", date)
		for _, tv := range toolVersions {
			fmt.Printf("%s=%s\n", strings.ToLower(tv.Component), tv.Version)
		}
		return nil
	}

	// Invalid format
	return fmt.Errorf("invalid format: %s (must be: json, plain)", format)
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

func showDevRequirements(format string) error {
	// Development requirements from COMPATIBILITY.md
	type requirement struct {
		Component string `json:"component"`
		Version   string `json:"version"`
		Required  bool   `json:"required"`
		Notes     string `json:"notes,omitempty"`
	}

	requirements := []requirement{
		{"e5s", "0.1.0", true, ""},
		{"Go", "go1.25.3", true, ""},
		{"go-spiffe SDK", "v2.6.0", true, ""},
		{"Docker", "v28.5.2", true, "Container runtime"},
		{"kubectl", "v1.33.4", true, "Kubernetes CLI"},
		{"Helm", "v3.18.6", true, "Package manager"},
		{"Minikube", "v1.37.0", true, "Local Kubernetes"},
		{"golangci-lint", "v1.64.8", true, "Linter"},
		{"govulncheck", "latest", true, "Vulnerability scanner"},
		{"gosec", "latest", false, "Security scanner"},
		{"gitleaks", "latest", false, "Secret scanner"},
	}

	// JSON output
	if format == "json" {
		type devOutput struct {
			Mode         string        `json:"mode"`
			Description  string        `json:"description"`
			Requirements []requirement `json:"requirements"`
			Installation []string      `json:"installation"`
		}

		output := devOutput{
			Mode:         "dev",
			Description:  "Tested versions for e5s development. Newer versions may work but are not guaranteed.",
			Requirements: requirements,
			Installation: []string{
				"Ubuntu 24.04: ./scripts/install-tools-ubuntu-24.04.sh",
				"macOS: brew install go docker kubectl helm minikube golangci-lint",
			},
		}

		jsonBytes, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	// Plain text output
	if format == "plain" {
		fmt.Println("# Development Requirements")
		for _, req := range requirements {
			status := "required"
			if !req.Required {
				status = "optional"
			}
			fmt.Printf("%s=%s (%s)\n", strings.ToLower(strings.ReplaceAll(req.Component, " ", "_")), req.Version, status)
		}
		return nil
	}

	// Invalid format
	return fmt.Errorf("invalid format: %s (must be: json, plain)", format)
}

func showProdRequirements(format string) error {
	// Production requirements from COMPATIBILITY.md
	type requirement struct {
		Component string `json:"component"`
		Version   string `json:"version"`
		Required  bool   `json:"required"`
		Notes     string `json:"notes,omitempty"`
	}

	requirements := []requirement{
		{"e5s", "0.1.0", true, ""},
		{"Go", "go1.25.3", true, "Runtime"},
		{"kubectl", "v1.33.4", true, "Kubernetes CLI"},
		{"Helm", "v3.18.6", true, "Package manager"},
		{"SPIRE Helm Chart", "v0.27.0", true, "spiffe/helm-charts-hardened"},
		{"SPIRE Server", "v1.13.0", true, "Via Helm chart"},
		{"SPIRE Agent", "v1.13.0", true, "Via Helm chart"},
		{"Kubernetes", "v1.28+", true, "Cluster version"},
	}

	// JSON output
	if format == "json" {
		type prodOutput struct {
			Mode          string        `json:"mode"`
			Description   string        `json:"description"`
			Requirements  []requirement `json:"requirements"`
			Deployment    []string      `json:"deployment"`
			Documentation []string      `json:"documentation"`
		}

		output := prodOutput{
			Mode:         "prod",
			Description:  "Tested versions for e5s production deployment. Newer versions may work but are not guaranteed.",
			Requirements: requirements,
			Deployment: []string{
				"Install SPIRE using Helm: helm install spire spiffe/spire --set 'spire-server.image.tag=1.13.0'",
				"Deploy your application with e5s library",
				"See examples/highlevel/ for deployment patterns",
			},
			Documentation: []string{
				"COMPATIBILITY.md - Full version compatibility matrix",
				"examples/ - Deployment examples",
			},
		}

		jsonBytes, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal JSON: %w", err)
		}
		fmt.Println(string(jsonBytes))
		return nil
	}

	// Plain text output
	if format == "plain" {
		fmt.Println("# Production Requirements")
		for _, req := range requirements {
			status := "required"
			if !req.Required {
				status = "optional"
			}
			fmt.Printf("%s=%s (%s)\n", strings.ToLower(strings.ReplaceAll(req.Component, " ", "_")), req.Version, status)
		}
		return nil
	}

	// Invalid format
	return fmt.Errorf("invalid format: %s (must be: json, plain)", format)
}
