package main

import (
	"flag"
	"fmt"

	"github.com/sufield/e5s/internal/config"
)

func validateCommand(args []string) error {
	fs := flag.NewFlagSet("validate", flag.ExitOnError)
	mode := fs.String("mode", "auto", "Validation mode: auto, server, client")

	fs.Usage = func() {
		fmt.Println(`Validate e5s configuration files

USAGE:
    e5s validate <config-file> [flags]

FLAGS:
    --mode string   Validation mode: auto, server, client (default "auto")

MODES:
    auto     Auto-detect configuration type and validate accordingly
    server   Validate as server configuration
    client   Validate as client configuration

EXAMPLES:
    # Validate configuration (auto-detect mode)
    e5s validate e5s-server.yaml

    # Validate as server configuration
    e5s validate e5s-server.yaml --mode server

    # Validate as client configuration
    e5s validate e5s-client.yaml --mode client

    # Use in CI/CD pipelines
    if e5s validate config/production.yaml; then
        echo "Configuration is valid"
        kubectl apply -f deployment.yaml
    fi`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("config file path required")
	}

	configPath := fs.Arg(0)

	// Validate based on mode
	switch *mode {
	case "auto":
		return validateAuto(configPath)
	case "server":
		return validateServerFile(configPath)
	case "client":
		return validateClientFile(configPath)
	default:
		return fmt.Errorf("invalid mode: %s (use 'auto', 'server', or 'client')", *mode)
	}
}

func validateAuto(path string) error {
	// Try loading as server config first
	serverCfg, serverErr := config.LoadServerConfig(path)
	if serverErr == nil {
		// Validate server config
		_, _, err := config.ValidateServerConfig(&serverCfg)
		if err == nil {
			fmt.Println("✓ Detected server configuration")
			return printServerConfig(&serverCfg, path)
		}
	}

	// Try loading as client config
	clientCfg, clientErr := config.LoadClientConfig(path)
	if clientErr == nil {
		// Validate client config
		_, _, err := config.ValidateClientConfig(&clientCfg)
		if err == nil {
			fmt.Println("✓ Detected client configuration")
			return printClientConfig(&clientCfg, path)
		}
	}

	// Both failed, return helpful error
	return fmt.Errorf("failed to load config as server or client:\n  Server: %v\n  Client: %v", serverErr, clientErr)
}

func validateServerFile(path string) error {
	cfg, err := config.LoadServerConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load server config: %w", err)
	}

	_, _, err = config.ValidateServerConfig(&cfg)
	if err != nil {
		return fmt.Errorf("server validation failed: %w", err)
	}

	return printServerConfig(&cfg, path)
}

func validateClientFile(path string) error {
	cfg, err := config.LoadClientConfig(path)
	if err != nil {
		return fmt.Errorf("failed to load client config: %w", err)
	}

	_, _, err = config.ValidateClientConfig(&cfg)
	if err != nil {
		return fmt.Errorf("client validation failed: %w", err)
	}

	return printClientConfig(&cfg, path)
}

func printServerConfig(cfg *config.ServerFileConfig, path string) error {
	fmt.Printf("✓ Valid server configuration: %s\n", path)
	fmt.Println("\nServer settings:")
	fmt.Printf("  Listen address: %s\n", cfg.Server.ListenAddr)

	if cfg.Server.AllowedClientSPIFFEID != "" {
		fmt.Printf("  Authorization: Specific SPIFFE ID\n")
		fmt.Printf("    Allowed client: %s\n", cfg.Server.AllowedClientSPIFFEID)
		fmt.Println("  Security level: ✓ Zero-trust (recommended)")
	} else if cfg.Server.AllowedClientTrustDomain != "" {
		fmt.Printf("  Authorization: Trust domain\n")
		fmt.Printf("    Allowed domain: %s\n", cfg.Server.AllowedClientTrustDomain)
		fmt.Println("  Security level: ⚠ Permissive (use specific SPIFFE ID for production)")
	}

	fmt.Printf("\nSPIRE settings:\n")
	fmt.Printf("  Workload socket: %s\n", cfg.SPIRE.WorkloadSocket)
	if cfg.SPIRE.InitialFetchTimeout != "" {
		fmt.Printf("  Initial fetch timeout: %s\n", cfg.SPIRE.InitialFetchTimeout)
	}

	return nil
}

func printClientConfig(cfg *config.ClientFileConfig, path string) error {
	fmt.Printf("✓ Valid client configuration: %s\n", path)
	fmt.Println("\nClient settings:")
	fmt.Println("  Note: Server URL should be passed via SERVER_URL env var or -url flag")

	if cfg.Client.ExpectedServerSPIFFEID != "" {
		fmt.Printf("  Server verification: Specific SPIFFE ID\n")
		fmt.Printf("    Expected server: %s\n", cfg.Client.ExpectedServerSPIFFEID)
		fmt.Println("  Security level: ✓ Zero-trust (recommended)")
	} else if cfg.Client.ExpectedServerTrustDomain != "" {
		fmt.Printf("  Server verification: Trust domain\n")
		fmt.Printf("    Expected domain: %s\n", cfg.Client.ExpectedServerTrustDomain)
		fmt.Println("  Security level: ⚠ Permissive (use specific SPIFFE ID for production)")
	}

	fmt.Printf("\nSPIRE settings:\n")
	fmt.Printf("  Workload socket: %s\n", cfg.SPIRE.WorkloadSocket)
	if cfg.SPIRE.InitialFetchTimeout != "" {
		fmt.Printf("  Initial fetch timeout: %s\n", cfg.SPIRE.InitialFetchTimeout)
	}

	return nil
}
