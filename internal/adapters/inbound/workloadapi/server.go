package workloadapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"syscall"

	"github.com/pocket/hexagon/spire/internal/app"
	"github.com/pocket/hexagon/spire/internal/domain"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Server is the Workload API HTTP server (inbound adapter)
// In production SPIRE, this would be gRPC over Unix domain socket
// This walking skeleton uses HTTP over Unix socket for simplicity
//
// Architecture note: This server extracts calling process credentials
// and delegates to IdentityClientService for SVID issuance
type Server struct {
	service      *app.IdentityClientService
	socketPath   string
	httpServer   *http.Server
	listener     net.Listener
}

// NewServer creates a new Workload API server
func NewServer(service *app.IdentityClientService, socketPath string) *Server {
	return &Server{
		service:    service,
		socketPath: socketPath,
	}
}

// Start starts the Workload API server on Unix domain socket
func (s *Server) Start(ctx context.Context) error {
	// Remove existing socket if present
	if err := os.RemoveAll(s.socketPath); err != nil {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix domain socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create Unix socket listener: %w", err)
	}
	s.listener = listener

	// Set socket permissions (readable/writable by all - workloads need access)
	if err := os.Chmod(s.socketPath, 0777); err != nil {
		listener.Close()
		return fmt.Errorf("failed to set socket permissions: %w", err)
	}

	// Create HTTP server with handler
	mux := http.NewServeMux()
	mux.HandleFunc("/svid/x509", s.handleFetchX509SVID)

	s.httpServer = &http.Server{
		Handler: mux,
	}

	// Start serving in background
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Printf("Workload API server error: %v\n", err)
		}
	}()

	fmt.Printf("Workload API listening on %s\n", s.socketPath)
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
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract calling process credentials from Unix socket connection
	// This is the key security mechanism - we attest the caller based on socket peer credentials
	callerIdentity, err := s.extractCallerIdentity(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to extract caller identity: %v", err), http.StatusInternalServerError)
		return
	}

	// Call the Identity Client service (core use case)
	identity, err := s.service.FetchX509SVIDForCaller(r.Context(), callerIdentity)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch SVID: %v", err), http.StatusInternalServerError)
		return
	}

	// Serialize and return identity document
	response := &X509SVIDResponse{
		SPIFFEID:    identity.IdentityNamespace.String(),
		X509SVID:    formatCertificate(identity.IdentityDocument),
		ExpiresAt:   identity.IdentityDocument.ExpiresAt().Unix(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, fmt.Sprintf("Failed to encode response: %v", err), http.StatusInternalServerError)
		return
	}
}

// extractCallerIdentity extracts process credentials from Unix socket connection
// This uses SO_PEERCRED to get the UID/PID/GID of the calling process
func (s *Server) extractCallerIdentity(r *http.Request) (ports.ProcessIdentity, error) {
	// For HTTP over Unix socket in Go's http.Server, extracting peer credentials
	// requires accessing the underlying connection. This is a known limitation.
	//
	// Workaround for walking skeleton: clients send their credentials in headers
	// Production SPIRE uses gRPC with custom connection handling to extract credentials
	//
	// Alternative: Use a custom net.Listener that wraps connections to store credentials

	// For demonstration: extract from headers (client must send truthfully for demo)
	// In production, this MUST use SO_PEERCRED - no trust in client-provided data
	var uid, pid, gid int
	if _, err := fmt.Sscanf(r.Header.Get("X-Spire-Caller-UID"), "%d", &uid); err != nil {
		uid = os.Getuid() // Fallback for demo
	}
	if _, err := fmt.Sscanf(r.Header.Get("X-Spire-Caller-PID"), "%d", &pid); err != nil {
		pid = os.Getpid() // Fallback for demo
	}
	if _, err := fmt.Sscanf(r.Header.Get("X-Spire-Caller-GID"), "%d", &gid); err != nil {
		gid = os.Getgid() // Fallback for demo
	}

	path := r.Header.Get("X-Spire-Caller-Path")
	if path == "" {
		path = "/proc/self/exe"
	}

	// NOTE: This header-based approach is ONLY for walking skeleton demonstration
	// Production implementation MUST use SO_PEERCRED or equivalent OS mechanism
	// See extractCallerCredentials() below for proper implementation pattern

	return ports.ProcessIdentity{
		PID:  pid,
		UID:  uid,
		GID:  gid,
		Path: path,
	}, nil
}

// extractCallerCredentials uses SO_PEERCRED to get Unix socket peer credentials
// This is commented out for now - would be used in production
func extractCallerCredentials(conn net.Conn) (*syscall.Ucred, error) {
	unixConn, ok := conn.(*net.UnixConn)
	if !ok {
		return nil, fmt.Errorf("connection is not a Unix socket")
	}

	// Get raw file descriptor
	file, err := unixConn.File()
	if err != nil {
		return nil, fmt.Errorf("failed to get file descriptor: %w", err)
	}
	defer file.Close()

	// Get peer credentials using SO_PEERCRED
	cred, err := syscall.GetsockoptUcred(int(file.Fd()), syscall.SOL_SOCKET, syscall.SO_PEERCRED)
	if err != nil {
		return nil, fmt.Errorf("failed to get peer credentials: %w", err)
	}

	return cred, nil
}

// X509SVIDResponse is the response format for X.509 SVID requests
type X509SVIDResponse struct {
	SPIFFEID    string `json:"spiffe_id"`
	X509SVID    string `json:"x509_svid"`
	ExpiresAt   int64  `json:"expires_at"`
}

// formatCertificate formats a certificate for response
// In production, this would return PEM-encoded certificate chain
func formatCertificate(doc *domain.IdentityDocument) string {
	if doc == nil {
		return ""
	}
	return fmt.Sprintf("X.509 Certificate for %s (expires: %s)",
		doc.IdentityNamespace().String(),
		doc.ExpiresAt().Format("2006-01-02 15:04:05"))
}
var _ ports.WorkloadAPIServer = (*Server)(nil)
