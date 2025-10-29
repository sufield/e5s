package spire

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

// Source provides SPIFFE X.509 identities from the SPIRE Workload API.
//
// This source:
//   - Connects to the SPIRE Agent's Workload API socket
//   - Maintains a live X509Source that automatically rotates certificates
//   - Provides thread-safe access to current certificates and trust bundles
//
// The X509Source watches the Workload API and updates certificates/bundles
// automatically, enabling zero-downtime rotation.
//
// Trust Domain Federation:
//
//	This implementation currently only supports single trust domain deployments.
//	The underlying SDK source returns the CA bundle for the workload's own trust domain only.
//	If you need cross-trust-domain mTLS (federation), configure SPIRE federation
//	and use go-spiffe's federated bundle APIs
//
// Lifecycle:
//   - Create once per process (or trust domain)
//   - Share across multiple TLS configs (servers and clients)
//   - Call Close() when done to release connections and goroutines
//
// Thread-safety: All methods are safe for concurrent use.
// Internal state (the underlying workloadapi.X509Source) is guarded by mu
// and becomes nil after Close(); callers must handle "source is closed" errors.
type Source struct {
	mu     sync.RWMutex
	source *workloadapi.X509Source

	// Close coordination
	closeOnce sync.Once
	closeErr  error
}

// Config configures the SPIRE cert source.
type Config struct {
	// WorkloadSocket is the path to the SPIRE agent's Workload API socket.
	//
	// If empty, the SDK auto-detects from SPIFFE_ENDPOINT_SOCKET environment variable.
	//
	// Accepts:
	//   - unix:// scheme: "unix:///tmp/spire-agent/public/api.sock"
	//   - tcp:// scheme: "tcp://spire-agent:8081"
	//   - Bare filesystem path: "/tmp/spire-agent/public/api.sock" (treated as unix://)
	WorkloadSocket string
}

// NewSource creates a new SPIRE-backed certificate source.
//
// This connects to the SPIRE Workload API and starts watching for certificate
// and trust bundle updates. The connection remains active until Close() is called.
//
// Socket path resolution:
//   - If cfg.WorkloadSocket is provided, uses that address
//   - If cfg.WorkloadSocket is empty, SDK auto-detects from SPIFFE_ENDPOINT_SOCKET
//   - Bare filesystem paths are converted to unix:// scheme
//
// The SDK's workloadapi.NewX509Source handles:
//   - Environment variable detection (SPIFFE_ENDPOINT_SOCKET)
//   - Initial connection and SVID fetch
//   - Starting background watchers for certificate rotation
//
// Returns error if:
//   - Context is nil
//   - SPIRE agent is not running or unreachable
//   - Workload is not registered with SPIRE
//   - Initial SVID fetch fails
//
// Context lifetime:
// ctx is ONLY used for the initial connection and first SVID/bundle fetch.
// After NewSource returns, the Source keeps running (watchers, rotation goroutines)
// even if ctx is canceled. To actually tear down identity, you MUST call Close().
// Canceling ctx does NOT stop background rotation.
func NewSource(ctx context.Context, cfg Config) (*Source, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	// Build SDK options
	var opts []workloadapi.X509SourceOption
	if cfg.WorkloadSocket != "" {
		// Normalize bare paths to unix:// scheme for SDK
		addr := normalizeToAddr(cfg.WorkloadSocket)
		opts = append(opts, workloadapi.WithClientOptions(workloadapi.WithAddr(addr)))
	}
	// If cfg.WorkloadSocket is empty, SDK will auto-detect from SPIFFE_ENDPOINT_SOCKET

	// Create X509Source using SDK
	// This:
	//   - Connects to the Workload API
	//   - Performs initial SVID/bundle fetch (blocks until ready or error)
	//   - Starts watching for updates in background
	x509src, err := workloadapi.NewX509Source(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509Source: %w", err)
	}

	return &Source{
		source: x509src,
	}, nil
}

// Close releases all resources (connections, watchers, goroutines).
//
// Closes the X509Source and releases all resources (connections, watchers, goroutines).
// After Close returns, the underlying SDK source is closed and cannot be used.
//
// Idempotent: safe to call multiple times. Subsequent calls return the cached error.
// X509Source returns the underlying SDK X509Source for use with SDK TLS helpers.
//
// The returned source implements both x509svid.Source and x509bundle.Source,
// which can be passed directly to tlsconfig.MTLSServerConfig() and similar SDK functions.
//
// Returns nil if the source has been closed.
func (s *Source) X509Source() *workloadapi.X509Source {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.source
}

func (s *Source) Close() error {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.source != nil {
			s.closeErr = s.source.Close()
			s.source = nil // Prevent use-after-close
		}
	})
	return s.closeErr
}

// normalizeToAddr converts a socket address to the scheme-prefixed format
// expected by the SDK's workloadapi.WithAddr.
//
// If the input already has a scheme (unix:// or tcp://), returns it unchanged.
// If it's a bare filesystem path, prefixes it with "unix://".
//
// Examples:
//   - "unix:///tmp/agent.sock" → "unix:///tmp/agent.sock" (unchanged)
//   - "tcp://agent:8081" → "tcp://agent:8081" (unchanged)
//   - "/tmp/agent.sock" → "unix:///tmp/agent.sock" (prefixed)
func normalizeToAddr(raw string) string {
	if strings.HasPrefix(raw, "unix://") || strings.HasPrefix(raw, "tcp://") {
		return raw
	}
	return "unix://" + raw
}

