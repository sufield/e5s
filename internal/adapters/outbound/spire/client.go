package spire

import (
	"context"
	"fmt"
	"time"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// SPIREClient provides access to the SPIRE Workload API for production environments.
// It implements the outbound port interfaces for identity management.
type SPIREClient struct {
	client      *workloadapi.Client
	socketPath  string
	trustDomain string
	timeout     time.Duration
}

// Config holds configuration for the SPIRE client
type Config struct {
	// SocketPath is the Unix domain socket path for the SPIRE agent
	// Default: /tmp/spire-agent/public/api.sock
	SocketPath string

	// TrustDomain is the SPIFFE trust domain
	// Example: example.org
	TrustDomain string

	// Timeout for Workload API operations
	// Default: 30s
	Timeout time.Duration
}

// NewSPIREClient creates a new SPIRE client connected to the Workload API
func NewSPIREClient(ctx context.Context, cfg Config) (*SPIREClient, error) {
	if cfg.SocketPath == "" {
		cfg.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	if cfg.TrustDomain == "" {
		return nil, fmt.Errorf("trust domain is required")
	}

	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Create workload API client
	client, err := workloadapi.New(ctx, workloadapi.WithAddr(cfg.SocketPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE workload API client: %w", err)
	}

	return &SPIREClient{
		client:      client,
		socketPath:  cfg.SocketPath,
		trustDomain: cfg.TrustDomain,
		timeout:     cfg.Timeout,
	}, nil
}

// Close closes the connection to the SPIRE Workload API
func (c *SPIREClient) Close() error {
	if c.client != nil {
		return c.client.Close()
	}
	return nil
}

// GetTrustDomain returns the configured trust domain
func (c *SPIREClient) GetTrustDomain() string {
	return c.trustDomain
}

// GetSocketPath returns the configured socket path
func (c *SPIREClient) GetSocketPath() string {
	return c.socketPath
}
