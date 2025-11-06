package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/sufield/e5s"
)

// Version information (set via ldflags during build)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// AppConfig represents the client application configuration.
// This demonstrates the real-world pattern: applications manage their own config,
// separate from the e5s library config.
type AppConfig struct {
	ServerURL string `yaml:"server_url"`
}

func main() {
	versionFlag := flag.Bool("version", false, "Print version information and exit")
	// Application config (server URL, etc.)
	appConfigPath := flag.String("app-config", "examples/highlevel/client-config.yaml", "Path to app config file")
	// e5s library config (SPIRE socket, trust domain, etc.)
	e5sConfigPath := flag.String("e5s-config", "examples/highlevel/e5s.yaml", "Path to e5s config file")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("e5s-example-client %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		os.Exit(0)
	}

	log.Printf("e5s mTLS client (version %s)", version)
	log.Printf("App config: %s", *appConfigPath)
	log.Printf("e5s config: %s", *e5sConfigPath)

	os.Exit(run(*appConfigPath, *e5sConfigPath))
}

func run(appConfigPath, e5sConfigPath string) int {
	// Load application-specific configuration
	appCfg, err := loadAppConfig(appConfigPath)
	if err != nil {
		log.Printf("❌ Failed to load app config: %v", err)
		return 1
	}

	// Allow SERVER_URL environment variable to override config
	// This is useful for Kubernetes deployments
	serverURL := os.Getenv("SERVER_URL")
	if serverURL == "" {
		serverURL = appCfg.ServerURL
	}
	if serverURL == "" {
		log.Printf("❌ server_url not set in config and SERVER_URL environment variable not set")
		return 1
	}

	// Set E5S_CONFIG for the library to use
	os.Setenv("E5S_CONFIG", e5sConfigPath)

	// Perform mTLS GET using high-level API
	// e5s.Get() handles client creation, SPIRE connection, and cleanup
	resp, err := e5s.Get(serverURL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	// Read and print response
	body, _ := io.ReadAll(resp.Body)
	log.Printf("← Server response: %s", string(body))
	fmt.Print(string(body))

	return 0
}

// loadAppConfig loads the application-specific configuration.
// This demonstrates the real-world pattern: applications manage their own config files.
func loadAppConfig(path string) (*AppConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
