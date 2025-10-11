package workloadapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Server is the Workload API HTTP server (inbound adapter)
// In production SPIRE, this would be gRPC over Unix domain socket
// This implementation uses HTTP over Unix socket with SO_PEERCRED attestation
//
// Architecture note: This server extracts calling process credentials
// and delegates to IdentityClientService for SVID issuance
type Server struct {
	service         *app.IdentityClientService
	socketPath      string
	socketPerm      os.FileMode // Socket file permissions (default: 0700 for production security)
	logger          *slog.Logger
	httpServer      *http.Server
	listener        net.Listener
}

// ServerOption configures the Workload API server
type ServerOption func(*Server)

// WithSocketPermissions sets the Unix socket file permissions
// Common values:
//   - 0700: Production owner-only (only owner can connect) [DEFAULT]
//   - 0770: Production group-access (owner + group can connect)
//   - 0777: Development/testing (any user can connect)
func WithSocketPermissions(perm os.FileMode) ServerOption {
	return func(s *Server) {
		s.socketPerm = perm
	}
}

// WithLogger sets a structured logger for the server
// If logger is nil, uses io.Discard for silent operation
func WithLogger(logger *slog.Logger) ServerOption {
	return func(s *Server) {
		if logger != nil {
			s.logger = logger
		} else {
			// If explicitly set to nil, use discard handler for silent operation
			s.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
		}
	}
}

// NewServer creates a new Workload API server
// Default socket permissions: 0700 (production security - owner-only access)
// For development/testing with multiple users, use WithSocketPermissions(0777)
// Default logger: stderr with Info level for production observability
func NewServer(service *app.IdentityClientService, socketPath string, opts ...ServerOption) *Server {
	s := &Server{
		service:    service,
		socketPath: socketPath,
		socketPerm: 0700, // Default: owner-only for production security
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
			Level: slog.LevelInfo, // Default: Info level for production observability
		})),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	return s
}

// Start starts the Workload API server on Unix domain socket
func (s *Server) Start(ctx context.Context) error {
	// Create socket directory with secure permissions if it doesn't exist
	// This ensures the socket is placed in a directory with appropriate access control
	socketDir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(socketDir, 0700); err != nil {
		return fmt.Errorf("failed to create socket directory %q: %w", socketDir, err)
	}

	// Enforce secure permissions even if directory already existed
	// This prevents security issues from pre-existing directories with lax permissions
	if info, err := os.Stat(socketDir); err == nil {
		if info.Mode().Perm() != 0700 {
			if err := os.Chmod(socketDir, 0700); err != nil {
				return fmt.Errorf("failed to set directory permissions to 0700: %w", err)
			}
			s.logger.Info("updated socket directory permissions",
				"dir", socketDir,
				"old_perms", fmt.Sprintf("%04o", info.Mode().Perm()),
				"new_perms", "0700")
		}
	}

	// Remove existing socket if present
	if err := os.RemoveAll(s.socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix domain socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket listener: %w", err)
	}

	// Wrap listener to extract kernel-verified credentials on connection accept
	// This uses SO_PEERCRED on Linux to get PID/UID/GID from the kernel
	credListener := newCredentialsListener(listener, s.logger)
	s.listener = credListener

	// Set socket permissions (configurable via WithSocketPermissions)
	// Default: 0777 (development), Production: 0700 (owner-only)
	if err := os.Chmod(s.socketPath, s.socketPerm); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions to %04o: %w", s.socketPerm, err)
	}

	// Create HTTP server with handler
	mux := http.NewServeMux()
	mux.HandleFunc("/svid/x509", s.handleFetchX509SVID)

	s.httpServer = &http.Server{
		Handler: mux,
		// ConnContext injects credentials from connection into request context
		ConnContext: credentialsConnContext,
	}

	// Start serving in background
	go func() {
		if err := s.httpServer.Serve(credListener); err != nil && err != http.ErrServerClosed {
			s.logger.Error("workload API server error",
				"error", err,
				"socket", s.socketPath)
		}
	}()

	s.logger.Info("workload API listening",
		"socket", s.socketPath,
		"permissions", fmt.Sprintf("%04o", s.socketPerm))
	return nil
}

// Stop gracefully stops the Workload API server
func (s *Server) Stop(ctx context.Context) error {
	if s.httpServer != nil {
		if err := s.httpServer.Shutdown(ctx); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
	}
	if s.listener != nil {
		s.listener.Close()
	}
	// Clean up socket file
	os.RemoveAll(s.socketPath)
	return nil
}

// handleFetchX509SVID handles the X.509 SVID fetch request
func (s *Server) handleFetchX509SVID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.logger.Warn("invalid HTTP method",
			"method", r.Method,
			"expected", http.MethodGet)
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract calling process credentials from Unix socket connection
	// This is the key security mechanism - we attest the caller based on socket peer credentials
	callerIdentity, err := s.extractCallerIdentity(r)
	if err != nil {
		s.logger.Error("failed to extract caller identity",
			"remote_addr", r.RemoteAddr,
			"error", err)
		http.Error(w, fmt.Sprintf("Failed to extract caller identity: %v", err), http.StatusInternalServerError)
		return
	}

	s.logger.Debug("authenticated workload",
		"remote_addr", r.RemoteAddr,
		"pid", callerIdentity.PID,
		"uid", callerIdentity.UID,
		"gid", callerIdentity.GID,
		"path", callerIdentity.Path)

	// Call the Identity Client service (core use case)
	identity, err := s.service.FetchX509SVIDForCaller(r.Context(), callerIdentity)
	if err != nil {
		s.logger.Error("failed to fetch SVID",
			"remote_addr", r.RemoteAddr,
			"error", err,
			"caller_pid", callerIdentity.PID,
			"caller_uid", callerIdentity.UID,
			"caller_path", callerIdentity.Path)
		http.Error(w, fmt.Sprintf("Failed to fetch SVID: %v", err), http.StatusInternalServerError)
		return
	}

	s.logger.Info("issued SVID",
		"remote_addr", r.RemoteAddr,
		"spiffe_id", identity.IdentityCredential.String(),
		"caller_pid", callerIdentity.PID,
		"caller_uid", callerIdentity.UID)

	// Serialize and return identity document
	response := &X509SVIDResponse{
		SPIFFEID:  identity.IdentityCredential.String(),
		X509SVID:  formatCertificate(identity.IdentityDocument),
		ExpiresAt: identity.IdentityDocument.ExpiresAt().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// extractCallerIdentity extracts kernel-verified process credentials from request context.
//
// Security: Credentials are extracted by the listener using SO_PEERCRED (Linux) at connection
// accept time. The kernel verifies these credentials, making them impossible to forge by the caller.
//
// This is a significant improvement over header-based attestation, which trusts client-provided data.
//
// Platform Support:
//   - Linux: Uses SO_PEERCRED (fully implemented)
//   - Other platforms: Returns error (requires platform-specific implementation)
func (s *Server) extractCallerIdentity(r *http.Request) (ports.ProcessIdentity, error) {
	// First check if there was an error during credential extraction
	// This catches cases where the connection was not properly wrapped
	if credErr, ok := credentialsErrorFromContext(r.Context()); ok {
		s.logger.Error("credential setup error",
			"error", credErr,
			"remote_addr", r.RemoteAddr)
		return ports.ProcessIdentity{}, fmt.Errorf("credential setup failed: %w", credErr)
	}

	// Retrieve kernel-verified credentials from request context
	// These were injected by credentialsConnContext during connection setup
	credentials, ok := credentialsFromContext(r.Context())
	if !ok {
		return ports.ProcessIdentity{}, fmt.Errorf(
			"peer credentials not found in request context; " +
				"this may indicate the connection was not properly wrapped or " +
				"the platform does not support kernel-verified credential extraction",
		)
	}

	return credentials, nil
}

// X509SVIDResponse is the response format for X.509 SVID requests
type X509SVIDResponse struct {
	SPIFFEID  string `json:"spiffe_id"`
	X509SVID  string `json:"x509_svid"`
	ExpiresAt int64  `json:"expires_at"`
}

// formatCertificate formats a certificate for response
// In production, this would return PEM-encoded certificate chain
func formatCertificate(doc *domain.IdentityDocument) string {
	if doc == nil {
		return ""
	}
	return fmt.Sprintf("X.509 Certificate for %s (expires: %s)",
		doc.IdentityCredential().String(),
		doc.ExpiresAt().Format("2006-01-02 15:04:05"))
}

var _ ports.WorkloadAPIServer = (*Server)(nil)
