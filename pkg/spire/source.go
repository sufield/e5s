package spire

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

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

	// InitialFetchTimeout is how long to wait for the first SVID/Bundle from
	// the Workload API before giving up and failing startup.
	//
	// This timeout applies to:
	//   - Connecting to the Workload API socket (dial timeout)
	//   - Fetching the first SVID and bundle (initial fetch timeout)
	//
	// If zero, a reasonable default will be used (30 seconds).
	// Set to a higher value in development environments where the SPIRE agent
	// may start slowly. Set to a lower value in production to fail fast.
	InitialFetchTimeout time.Duration
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
// ctx controls the lifetime of the X509Source's background watchers.
// The source will continue running (rotation, updates) until either:
//   - ctx is canceled, OR
//   - Close() is called
//
// InitialFetchTimeout (separate from ctx) bounds how long we wait for the
// first SVID before returning an error. This prevents hanging forever if
// SPIRE agent is unreachable.
func NewSource(ctx context.Context, cfg Config) (*Source, error) {
	if ctx == nil {
		return nil, errors.New("context cannot be nil")
	}

	// Determine timeout for initial fetch
	timeout := cfg.InitialFetchTimeout
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}

	// Build SDK options
	var opts []workloadapi.X509SourceOption
	if cfg.WorkloadSocket != "" {
		// Normalize bare paths to unix:// scheme for SDK
		addr := normalizeToAddr(cfg.WorkloadSocket)
		opts = append(opts, workloadapi.WithClientOptions(workloadapi.WithAddr(addr)))
	}
	// If cfg.WorkloadSocket is empty, SDK will auto-detect from SPIFFE_ENDPOINT_SOCKET

	// Create a cancellable context derived from the long-lived parent context.
	// This context will control the X509Source lifetime (rotation, watching).
	buildCtx, cancel := context.WithCancel(ctx)

	// Channel to receive the result from the goroutine
	type result struct {
		src *workloadapi.X509Source
		err error
	}
	ch := make(chan result, 1)

	// Start X509Source creation in a goroutine so we can timeout
	go func() {
		// NewX509Source blocks until first SVID is received
		src, err := workloadapi.NewX509Source(buildCtx, opts...)
		ch <- result{src, err}
	}()

	// Wait for either success or timeout
	select {
	case r := <-ch:
		if r.err != nil {
			cancel() // Abort partial startup
			return nil, fmt.Errorf("failed to create X509Source: %w", r.err)
		}
		// Success! buildCtx stays alive, controlled by parent ctx.
		// The source's watchers will run until ctx is canceled or Close() is called.
		return &Source{source: r.src}, nil

	case <-time.After(timeout):
		cancel() // Stop trying to build the source
		return nil, fmt.Errorf("initial SPIRE fetch timed out after %v", timeout)
	}
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
