package main

import (
	"fmt"
	"os"
)

// Version information (set via ldflags during build)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	// Create version info
	versionInfo := VersionInfo{
		Version: version,
		Commit:  commit,
		Date:    date,
	}

	// Create and configure registry
	registry := NewCommandRegistry(versionInfo)

	// Register all commands
	registerCommands(registry)

	// Execute
	if err := registry.Execute(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func registerCommands(r *CommandRegistry) {
	// Register spiffe-id command
	r.Register(&Command{
		Name:        "spiffe-id",
		Description: "Construct SPIFFE IDs from components",
		Usage:       "e5s spiffe-id <type> [arguments...] [flags]",
		Examples: []string{
			"e5s spiffe-id k8s default api-client",
			"e5s spiffe-id k8s default api-client --trust-domain=example.org",
			"e5s spiffe-id from-deployment ./k8s/client.yaml",
			"e5s spiffe-id custom example.org service api-server",
		},
		Run: spiffeIDCommand,
	})

	// Register discover command
	r.Register(&Command{
		Name:        "discover",
		Description: "Discover SPIFFE IDs from Kubernetes pods",
		Usage:       "e5s discover <resource-type> [name] [flags]",
		Examples: []string{
			"e5s discover trust-domain",
			"e5s discover pod e5s-client",
			"e5s discover label app=api-client --namespace production",
			"e5s discover deployment web-frontend",
		},
		Run: discoverCommand,
	})

	// Register validate command
	r.Register(&Command{
		Name:        "validate",
		Description: "Validate e5s configuration files",
		Usage:       "e5s validate <config-file> [flags]",
		Examples: []string{
			"e5s validate e5s.yaml",
			"e5s validate e5s.yaml --mode server",
		},
		Run: validateCommand,
	})

	// Register client command (data-plane debugging)
	r.Register(&Command{
		Name:        "client",
		Description: "Make mTLS requests using e5s (data-plane debugging)",
		Usage:       "e5s client <subcommand> [flags]",
		Examples: []string{
			"e5s client request --config ./e5s.yaml --url https://localhost:8443/time",
			"e5s client request --config ./e5s.yaml --url https://server:8443/api --debug",
		},
		Run: clientCommand,
	})

	// Register version command
	r.Register(&Command{
		Name:        "version",
		Description: "Show version information",
		Usage:       "e5s version [flags]",
		Examples: []string{
			"e5s version",
			"e5s version --mode dev",
			"e5s version --verbose",
		},
		Run: versionCommand,
	})

	// Register help command
	r.Register(&Command{
		Name:        "help",
		Description: "Show help information",
		Usage:       "e5s help [command]",
		Examples: []string{
			"e5s help",
			"e5s help spiffe-id",
		},
		Run: func(args []string) error {
			r.PrintHelp(os.Stdout)
			return nil
		},
	})
}
