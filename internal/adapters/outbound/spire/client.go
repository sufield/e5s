package spire

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// SPIREClient provides access to the SPIRE Workload API for production environments.
// It implements the outbound port interfaces for identity management.
//
// Design Note: This client wraps the Workload API with two strategies:
// - source: Cached X509Source for auto-rotating bundles/SVIDs (preferred, no RPC per call)
// - client: Direct Workload API client (used only if source creation fails)
//
// The source is preferred because it:
// - Caches bundles and SVIDs in memory
// - Automatically rotates certificates before expiration
// - Avoids RPC overhead on every fetch operation
//
// Concurrency: Close() is safe to call multiple times concurrently using sync.Once.
type SPIREClient struct {
	client      *workloadapi.Client
	source      *workloadapi.X509Source // Cached, auto-rotating source (preferred)
	socketPath  string
	td          spiffeid.TrustDomain // Normalized trust domain (value type for safety)
	timeout     time.Duration

	// Close coordination
	closeOnce sync.Once
	closeErr  error
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

// NewSPIREClient creates a new SPIRE client connected to the Workload API.
//
// Lifecycle Note: This constructor uses context.Background() for the underlying
// Workload API connection to avoid tying the client lifetime to a short-lived
// caller context. The client remains active until Close() is explicitly called.
// Individual operations (like FetchX509SVID) still respect per-operation timeouts.
//
// Parameters:
//   - ctx: Used only for initial connection validation (not stored)
//   - cfg: Client configuration (socket path, trust domain, timeout)
//
// Returns error if:
//   - Trust domain is empty or invalid DNS format
//   - Timeout is <= 0 (must be positive)
//   - Workload API connection fails
func NewSPIREClient(ctx context.Context, cfg Config) (*SPIREClient, error) {
	// Apply defaults
	if cfg.SocketPath == "" {
		cfg.SocketPath = "unix:///tmp/spire-agent/public/api.sock"
	}

	// Validate trust domain (not just empty check - validate DNS format)
	if cfg.TrustDomain == "" {
		return nil, fmt.Errorf("%w: trust domain cannot be empty", domain.ErrInvalidTrustDomain)
	}
	td, err := spiffeid.TrustDomainFromString(cfg.TrustDomain)
	if err != nil {
		return nil, fmt.Errorf("%w: invalid trust domain %q: %w", domain.ErrInvalidTrustDomain, cfg.TrustDomain, err)
	}

	// Validate timeout is positive
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}

	// Create workload API client with background context
	// This decouples client lifetime from caller's context
	client, err := workloadapi.New(context.Background(), workloadapi.WithAddr(cfg.SocketPath))
	if err != nil {
		return nil, fmt.Errorf("create workload API client: %w", err)
	}

	// Create X509Source for cached, auto-rotating bundles and SVIDs
	// This avoids RPC overhead on every fetch operation
	// Use timeout to avoid hanging if Workload API is unavailable
	sourceCtx, sourceCancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer sourceCancel()

	source, err := workloadapi.NewX509Source(sourceCtx, workloadapi.WithClient(client))
	if err != nil {
		// Source creation failed - close client and propagate error
		_ = client.Close()
		return nil, fmt.Errorf("create X509 source (Workload API may be unavailable): %w", err)
	}

	return &SPIREClient{
		client:     client,
		source:     source,
		socketPath: cfg.SocketPath,
		td:         td, // Store as value type for safety
		timeout:    cfg.Timeout,
	}, nil
}

// Close closes the X509Source and Workload API client.
//
// Idempotent: Safe to call multiple times concurrently. Uses sync.Once to ensure
// resources are closed exactly once. Returns errors.Join of both close operations,
// so you don't lose the second error if both fail.
//
// Concurrency: Thread-safe. Multiple goroutines can call Close concurrently without
// data races. The first call performs the actual close, subsequent calls return the
// cached result.
func (c *SPIREClient) Close() error {
	c.closeOnce.Do(func() {
		var err1, err2 error

		// Close source first (it wraps the client)
		if c.source != nil {
			err1 = c.source.Close()
		}

		// Then close underlying client
		if c.client != nil {
			err2 = c.client.Close()
		}

		// Join both errors (Go 1.20+)
		c.closeErr = errors.Join(err1, err2)
	})
	return c.closeErr
}

// GetTrustDomain returns the configured trust domain as a string.
// Returns the normalized (lowercase) form of the trust domain.
func (c *SPIREClient) GetTrustDomain() string {
	return c.td.String()
}

// TrustDomain returns the configured trust domain as a spiffeid.TrustDomain value type.
// Prefer this over GetTrustDomain() when working with go-spiffe APIs to avoid repeated
// string parsing and enable safer type-checked comparisons.
func (c *SPIREClient) TrustDomain() spiffeid.TrustDomain {
	return c.td
}

// GetSocketPath returns the configured socket path
func (c *SPIREClient) GetSocketPath() string {
	return c.socketPath
}

// GetWorkloadAPIClient exposes the underlying workloadapi.Client for advanced scenarios.
//
// WARNING: This method provides low-level access and is considered unstable API surface.
// Prefer using SPIREClient's higher-level methods when possible. Direct client access
// bypasses caching and may have performance implications.
//
// Use cases:
//   - Creating custom x509bundle.Source implementations
//   - Direct Workload API operations not exposed by SPIREClient
//
// Compatibility: No guarantees about API stability across versions.
func (c *SPIREClient) GetWorkloadAPIClient() *workloadapi.Client {
	return c.client
}

// Source returns the cached X509Source for read-only access.
//
// The X509Source provides cached, auto-rotating SVIDs and bundles. This is useful
// for advanced integrations that expect an x509svid.Source or x509bundle.Source
// interface without exposing the raw Workload API client.
//
// Returns nil if the source was not successfully created during NewSPIREClient.
func (c *SPIREClient) Source() *workloadapi.X509Source {
	return c.source
}

// GetX509BundleForTrustDomain fetches the X.509 bundle for a trust domain.
// This implements the x509bundle.Source interface requirement for certificate validation.
//
// Performance Note: This method uses the cached X509Source (if available) to avoid
// RPC overhead on every call. Bundles are automatically rotated by the source.
// Falls back to direct Workload API fetch if source is unavailable.
//
// Parameters:
//   - td: SPIFFE trust domain to fetch bundle for
//
// Returns:
//   - Bundle containing root CA certificates for the trust domain
//   - Error if trust domain has no bundle or fetch fails
func (c *SPIREClient) GetX509BundleForTrustDomain(td spiffeid.TrustDomain) (*x509bundle.Bundle, error) {
	// Helper to unwrap bundle result with consistent error formatting
	get := func(b *x509bundle.Bundle, err error) (*x509bundle.Bundle, error) {
		if err != nil {
			return nil, fmt.Errorf("trust bundle not found for domain %q: %w", td, err)
		}
		if b == nil {
			return nil, fmt.Errorf("trust bundle not found for domain %q", td)
		}
		return b, nil
	}

	// Prefer cached source (no RPC, auto-rotating)
	if c.source != nil {
		return get(c.source.GetX509BundleForTrustDomain(td))
	}

	// Fallback: Direct Workload API fetch (RPC per call)
	ctx, cancel := context.WithTimeout(context.Background(), c.timeout)
	defer cancel()

	bundleSet, err := c.client.FetchX509Bundles(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch X.509 bundles: %w", err)
	}

	return get(bundleSet.GetX509BundleForTrustDomain(td))
}

// Compile-time assertion: SPIREClient implements x509bundle.Source
var _ x509bundle.Source = (*SPIREClient)(nil)
