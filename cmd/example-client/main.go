package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

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
	e5sConfigPath := flag.String("e5s-config", "examples/highlevel/e5s-client.yaml", "Path to e5s config file")
	// Convenience flags for sshd-like debugging experience
	urlFlag := flag.String("url", "", "Server URL (overrides config)")
	debug := flag.Bool("debug", false, "Enable debug mode (verbose logging)")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("e5s-example-client %s\n", version)
		fmt.Printf("  commit: %s\n", commit)
		fmt.Printf("  built:  %s\n", date)
		os.Exit(0)
	}

	// Enable debug logging if requested
	if *debug {
		log.SetFlags(log.LstdFlags | log.Lmicroseconds | log.Lshortfile)
		if err := os.Setenv("E5S_DEBUG", "1"); err != nil {
			log.Printf("Warning: failed to set E5S_DEBUG: %v", err)
		}
		log.Println("⚠️  DEBUG MODE: verbose logging enabled")
	}

	log.Printf("e5s mTLS client (version %s)", version)
	log.Printf("App config: %s", *appConfigPath)
	log.Printf("e5s config: %s", *e5sConfigPath)

	os.Exit(run(*appConfigPath, *e5sConfigPath, *urlFlag))
}

func run(appConfigPath, e5sConfigPath, urlFlagValue string) int {
	// Load application-specific configuration
	appCfg, err := loadAppConfig(appConfigPath)
	if err != nil {
		log.Printf("❌ Failed to load app config: %v", err)
		return 1
	}

	// Determine server URL with priority: -url flag > SERVER_URL env var > config file
	serverURL := urlFlagValue
	if serverURL == "" {
		serverURL = os.Getenv("SERVER_URL")
	}
	if serverURL == "" {
		serverURL = appCfg.ServerURL
	}
	if serverURL == "" {
		log.Printf("❌ server_url not set in config, SERVER_URL env var, or -url flag")
		return 1
	}

	// example-start:client-request
	// Create mTLS HTTP client
	log.Println("→ Initializing SPIRE client and fetching SPIFFE identity...")
	client, shutdown, err := e5s.Client(e5sConfigPath)
	if err != nil {
		log.Printf("❌ Failed to create client: %v", err)
		return 1
	}
	defer func() {
		if err := shutdown(); err != nil {
			log.Printf("⚠️  Shutdown error: %v", err)
		}
	}()

	// Perform mTLS GET request
	log.Printf("→ Requesting from: %s", serverURL)
	resp, err := client.Get(serverURL)
	if err != nil {
		log.Printf("❌ Request failed: %v", err)
		return 1
	}
	defer resp.Body.Close()

	log.Printf("✓ Received response: HTTP %d %s", resp.StatusCode, resp.Status)

	// Read and print response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("❌ Failed to read response body: %v", err)
		return 1
	}
	log.Printf("← Server response: %s", string(body))
	fmt.Print(string(body))
	// example-end:client-request

	return 0
}

// loadAppConfig loads the application-specific configuration.
// This demonstrates the real-world pattern: applications manage their own config files.
func loadAppConfig(path string) (*AppConfig, error) {
	// Validate path to prevent directory traversal attacks
	cleanPath := filepath.Clean(path)
	if filepath.IsAbs(cleanPath) {
		return nil, fmt.Errorf("absolute paths not allowed")
	}
	if strings.Contains(cleanPath, "..") {
		return nil, fmt.Errorf("path traversal not allowed")
	}

	data, err := os.ReadFile(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}
