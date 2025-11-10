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
    e5s validate e5s.yaml

    # Validate as server configuration
    e5s validate e5s.yaml --mode server

    # Validate as client configuration
    e5s validate e5s.yaml --mode client

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

	// Load configuration
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Validate based on mode
	switch *mode {
	case "auto":
		return validateAuto(&cfg, configPath)
	case "server":
		return validateServer(&cfg, configPath)
	case "client":
		return validateClient(&cfg, configPath)
	default:
		return fmt.Errorf("invalid mode: %s (use 'auto', 'server', or 'client')", *mode)
	}
}

func validateAuto(cfg *config.FileConfig, path string) error {
	hasServer := cfg.Server.ListenAddr != "" ||
		cfg.Server.AllowedClientSPIFFEID != "" ||
		cfg.Server.AllowedClientTrustDomain != ""

	hasClient := cfg.Client.ExpectedServerSPIFFEID != "" ||
		cfg.Client.ExpectedServerTrustDomain != ""

	switch {
	case hasServer && hasClient:
		fmt.Printf("✓ Configuration appears to contain both server and client sections\n")
		fmt.Println("\nValidating server configuration:")
		if err := validateServer(cfg, path); err != nil {
			return err
		}
		fmt.Println("\nValidating client configuration:")
		return validateClient(cfg, path)
	case hasServer:
		fmt.Println("✓ Detected server configuration")
		return validateServer(cfg, path)
	case hasClient:
		fmt.Println("✓ Detected client configuration")
		return validateClient(cfg, path)
	default:
		return fmt.Errorf("configuration contains neither server nor client settings")
	}
}

func validateServer(cfg *config.FileConfig, path string) error {
	_, _, err := config.ValidateServer(cfg)
	if err != nil {
		return fmt.Errorf("server validation failed: %w", err)
	}

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

func validateClient(cfg *config.FileConfig, path string) error {
	_, _, err := config.ValidateClient(cfg)
	if err != nil {
		return fmt.Errorf("client validation failed: %w", err)
	}

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
