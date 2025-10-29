package spire

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	// If empty, auto-detects from:
	//   - SPIFFE_ENDPOINT_SOCKET env var
	//   - /tmp/spire-agent/public/api.sock
	//   - /var/run/spire/sockets/agent.sock
	WorkloadSocket string
}

// NewSource creates a new SPIRE-backed certificate source.
//
// This connects to the SPIRE Workload API and starts watching for certificate
// and trust bundle updates. The connection remains active until Close() is called.
//
// Socket path resolution (first non-empty wins):
//  1. cfg.WorkloadSocket (if provided)
//  2. SPIFFE_ENDPOINT_SOCKET environment variable (tcp:// or unix:// allowed)
//  3. /tmp/spire-agent/public/api.sock (common default)
//  4. /var/run/spire/sockets/agent.sock (alternate location)
//
// Returns error if:
//   - No socket path could be determined
//   - Socket does not exist or is not accessible
//   - SPIRE agent is not running
//   - Workload is not registered with SPIRE
//   - Initial SVID fetch fails
//
// The ctx parameter is used only for the initial connection and SVID fetch.
// The source continues running after this function returns until Close() is called.
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

	// Resolve socket path
	socketPath := cfg.WorkloadSocket
	if socketPath == "" {
		socketPath = detectSocketPath()
	}

	if socketPath == "" {
		return nil, errors.New("no SPIRE socket path configured and auto-detection failed")
	}

	// Normalize socket path to consistent format (unix:// or tcp:// prefix)
	socketPath = normalizeSocketAddr(socketPath)

	// Validate socket exists and is accessible
	if err := validateSocket(socketPath); err != nil {
		return nil, fmt.Errorf("invalid socket path %q: %w", socketPath, err)
	}

	// Create X509Source
	// This:
	//   - Connects to the Workload API
	//   - Fetches initial SVID and trust bundle
	//   - Starts watching for updates
	//
	// workloadapi.WithAddr expects full "unix://..." or "tcp://..." form.
	// We pass the same value we validated.
	x509src, err := workloadapi.NewX509Source(
		ctx,
		workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create X509Source: %w", err)
	}

	s := &Source{
		source: x509src,
	}
	return s, nil
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

// normalizeSocketAddr ensures socket addresses have a consistent format.
//
// If the input is a bare filesystem path (e.g. "/tmp/spire-agent/public/api.sock"),
// it's converted to unix:// format. If it already has a scheme (unix:// or tcp://),
// it's returned as-is.
//
// This prevents validation inconsistencies and makes the format predictable for
// workloadapi.WithAddr.
func normalizeSocketAddr(raw string) string {
	// Already has a scheme prefix
	if strings.HasPrefix(raw, "unix://") || strings.HasPrefix(raw, "tcp://") {
		return raw
	}
	// Bare filesystem path - treat as unix socket
	return "unix://" + raw
}

// detectSocketPath attempts to auto-detect the SPIRE agent socket path.
//
// If SPIFFE_ENDPOINT_SOCKET is set, we trust it as-is (unix://, tcp://, or bare path).
// We only proactively probe well-known unix:// defaults.
//
// Checks in order:
//  1. SPIFFE_ENDPOINT_SOCKET environment variable
//  2. /tmp/spire-agent/public/api.sock (common default)
//  3. /var/run/spire/sockets/agent.sock (alternate location)
//
// Returns empty string if no socket is found.
func detectSocketPath() string {
	// Try env var first (normalize in case it's a bare path)
	if socket := os.Getenv("SPIFFE_ENDPOINT_SOCKET"); socket != "" {
		return normalizeSocketAddr(socket)
	}

	// Try common socket paths
	commonPaths := []string{
		"unix:///tmp/spire-agent/public/api.sock",
		"unix:///var/run/spire/sockets/agent.sock",
	}

	for _, path := range commonPaths {
		if err := validateSocket(path); err == nil {
			return path
		}
	}

	return ""
}

// validateSocket checks if a socket path is valid and accessible.
//
// For unix:// sockets, verifies the file exists and is a socket.
//
// For non-unix schemes (e.g. tcp://host:port), we do not verify locality
// or existence here. We assume the caller intentionally pointed at a remote
// SPIRE endpoint. Allowing TCP here is a policy decision: tightening this
// is a breaking change.
func validateSocket(socketPath string) error {
	// If it starts with unix://, extract the actual filesystem path
	if strings.HasPrefix(socketPath, "unix://") {
		fsPath := socketPath[len("unix://"):]
		info, err := os.Stat(fsPath)
		if err != nil {
			return fmt.Errorf("socket does not exist: %w", err)
		}

		// Check if it's a socket (not a regular file or directory)
		if info.Mode()&os.ModeSocket == 0 {
			return fmt.Errorf("path exists but is not a socket")
		}
		return nil
	}

	// For tcp:// sockets, do basic sanity check without network reachability test
	if strings.HasPrefix(socketPath, "tcp://") {
		hostPort := strings.TrimPrefix(socketPath, "tcp://")
		if hostPort == "" || !strings.Contains(hostPort, ":") {
			return fmt.Errorf("tcp socket must be host:port, got %q", socketPath)
		}
		return nil
	}

	return fmt.Errorf("unsupported socket scheme (must be unix:// or tcp://): %q", socketPath)
}

