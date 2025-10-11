package spire

import (
	"context"
	"fmt"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
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
		return nil, domain.ErrInvalidTrustDomain
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

// GetWorkloadAPIClient returns the underlying workloadapi.Client for advanced operations.
// This can be used to access bundle sources for certificate validation.
func (c *SPIREClient) GetWorkloadAPIClient() *workloadapi.Client {
	return c.client
}

// GetX509BundleForTrustDomain fetches the X.509 bundle for a trust domain.
// This implements the x509bundle.Source interface requirement for certificate validation.
func (c *SPIREClient) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	// Fetch all bundles from Workload API
	bundleSet, err := c.client.FetchX509Bundles(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch X.509 bundles: %w", err)
	}

	// Get bundle for specific trust domain
	// GetX509BundleForTrustDomain returns (bundle, error) in newer SDK versions
	bundle, err := bundleSet.GetX509BundleForTrustDomain(td)
	if err != nil {
		return nil, fmt.Errorf("trust bundle not found for domain %s: %w", td, err)
	}

	return bundle, nil
}
