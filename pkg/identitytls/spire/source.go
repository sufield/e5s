package spire

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spiffe/go-spiffe/v2/workloadapi"

	"github.com/sufield/e5s/pkg/identitytls"
)

// Source implements identitytls.CertSource using the SPIRE Workload API.
//
// This source:
//   - Connects to the SPIRE Agent's Workload API socket
//   - Maintains a live X509Source that automatically rotates certificates
//   - Provides thread-safe access to current certificates and trust bundles
//
// The X509Source watches the Workload API and updates certificates/bundles
// automatically, enabling zero-downtime rotation.
//
// Certificate Requirements:
//
//	GetTLSCertificate always populates the Leaf field with the parsed X.509
//	certificate. This is required by identitytls.NewServerTLSConfig to extract
//	the server's SPIFFE ID and trust domain during initialization.
//
// Trust Domain Federation:
//
//	This implementation currently only supports single trust domain deployments.
//	GetRootCAs returns the CA bundle for the workload's own trust domain only.
//	If you need cross-trust-domain mTLS (federation), you must either:
//	  1. Configure SPIRE federation and use go-spiffe's federated bundle APIs, or
//	  2. Implement a custom CertSource that merges multiple trust domain bundles
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

// GetTLSCertificate implements identitytls.CertSource.
func (s *Source) GetTLSCertificate(_ context.Context) (tls.Certificate, error) {
	// Snapshot source under read lock to prevent race with Close()
	s.mu.RLock()
	src := s.source
	s.mu.RUnlock()

	if src == nil {
		return tls.Certificate{}, errors.New("source is closed")
	}

	// Get current SVID from the source
	// This returns the latest rotated certificate if rotation has occurred
	svid, err := src.GetX509SVID()
	if err != nil {
		return tls.Certificate{}, fmt.Errorf("failed to get X509 SVID: %w", err)
	}

	// Extract certificates and private key
	certs := svid.Certificates
	privateKey := svid.PrivateKey

	if len(certs) == 0 {
		return tls.Certificate{}, errors.New("SVID has no certificates")
	}

	if privateKey == nil {
		return tls.Certificate{}, errors.New("SVID has no private key")
	}

	// Build tls.Certificate
	// The Certificate field contains DER-encoded certificates (leaf first, then intermediates)
	certDER := make([][]byte, len(certs))
	for i, cert := range certs {
		certDER[i] = cert.Raw
	}

	return tls.Certificate{
		Certificate: certDER,
		PrivateKey:  privateKey,
		Leaf:        certs[0], // Cache parsed leaf for performance
	}, nil
}

// GetRootCAs implements identitytls.CertSource.
func (s *Source) GetRootCAs(_ context.Context) (*x509.CertPool, error) {
	// Snapshot source under read lock to prevent race with Close()
	s.mu.RLock()
	src := s.source
	s.mu.RUnlock()

	if src == nil {
		return nil, errors.New("source is closed")
	}

	// Get current SVID to determine our trust domain
	svid, err := src.GetX509SVID()
	if err != nil {
		return nil, fmt.Errorf("failed to get X509 SVID: %w", err)
	}

	// Get trust bundle from the source
	// This returns the latest bundle if it has been updated
	bundle, err := src.GetX509BundleForTrustDomain(svid.ID.TrustDomain())
	if err != nil {
		return nil, fmt.Errorf("failed to get X509 bundle: %w", err)
	}

	// Extract CA certificates from the bundle
	//
	// NOTE: We intentionally ONLY trust our own trust domain bundle here.
	// We do NOT add federated bundles. Expanding trust domains is a
	// security boundary change and must be an explicit API decision.
	// See the Source type's "Trust Domain Federation" doc for guidance.
	//
	// A fresh *x509.CertPool is returned on each call; callers may modify it.
	pool := x509.NewCertPool()
	for _, cert := range bundle.X509Authorities() {
		pool.AddCert(cert)
	}

	if len(pool.Subjects()) == 0 {
		return nil, errors.New("trust bundle has no CA certificates")
	}

	return pool, nil
}

// Close implements identitytls.CertSource.
//
// Closes the X509Source and releases all resources (connections, watchers, goroutines).
// After Close returns, future GetTLSCertificate/GetRootCAs calls return "source is closed".
//
// Idempotent: safe to call multiple times. Subsequent calls return the cached error.
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

// Compile-time assertion that Source implements identitytls.CertSource
var _ identitytls.CertSource = (*Source)(nil)
