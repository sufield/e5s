//go:build debug

package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	maxRequestBodyBytes = 10 * 1024 // 10KB max for fault injection requests
)

// Server is the debug HTTP server
type Server struct {
	addr         string
	mux          *http.ServeMux
	introspector Introspector
	httpServer   *http.Server
}

// FaultRequest represents a fault injection request
type FaultRequest struct {
	DropNextHandshake         *bool `json:"drop_next_handshake,omitempty"`
	CorruptNextSPIFFEID       *bool `json:"corrupt_next_spiffe_id,omitempty"`
	DelayNextIssueSeconds     *int  `json:"delay_next_issue_seconds,omitempty"`
	ForceTrustDomainMismatch  *bool `json:"force_trust_domain_mismatch,omitempty"`
	ForceExpiredCert          *bool `json:"force_expired_cert,omitempty"`
	RejectNextWorkloadLookup  *bool `json:"reject_next_workload_lookup,omitempty"`
}

// Start starts the debug HTTP server (debug build only).
// The server runs on localhost only and should never be exposed to external networks.
//
// The introspector parameter provides access to sanitized identity state.
// It can be nil, in which case the /_debug/identity endpoint will not be available.
//
// Returns the Server instance for graceful shutdown, or nil if the server was not started.
func Start(introspector Introspector) *Server {
	if !Active.LocalDebugServer {
		return nil
	}

	// Enforce loopback-only binding for security
	if Active.DebugServerAddr == "" {
		GetLogger().Debugf("REFUSING to start debug server: empty bind address")
		return nil
	}
	if !isLoopback(Active.DebugServerAddr) {
		// Clarify that non-loopback includes public IPs AND non-loopback hostnames.
		GetLogger().Debugf(
			"REFUSING to start debug server on non-loopback addr/host: %q (must be 127.0.0.0/8, ::1, or localhost)",
			Active.DebugServerAddr,
		)
		return nil
	}

	srv := &Server{
		addr:         Active.DebugServerAddr,
		mux:          http.NewServeMux(),
		introspector: introspector,
	}
	srv.registerHandlers()

	// Bind to the address before starting goroutine
	// This gives us the actual port when using :0 (ephemeral port)
	ln, err := net.Listen("tcp", srv.addr)
	if err != nil {
		GetLogger().Debugf("Failed to bind debug server: %v", err)
		return nil
	}

	// Create http.Server before starting goroutine so it can be shut down
	srv.httpServer = &http.Server{
		Addr:              ln.Addr().String(), // Use actual bound address
		Handler:           srv.mux,
		ReadHeaderTimeout: 2 * time.Second,  // Prevent Slowloris attacks
		IdleTimeout:       30 * time.Second, // Cap idle connection lifetime
		MaxHeaderBytes:    8 << 10,          // 8KB header limit
	}

	go func() {
		logger := GetLogger()
		// Safe to log: loopback-only address already validated by isLoopback().
		logger.Debugf("⚠️  DEBUG SERVER RUNNING ON %s", ln.Addr().String())
		logger.Debug("⚠️  WARNING: Debug mode is enabled. DO NOT USE IN PRODUCTION!")

		if err := srv.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Debugf("Debug server error: %v", err)
		}
	}()

	return srv
}

func (s *Server) registerHandlers() {
	s.mux.HandleFunc("/_debug/", s.handleIndex)
	s.mux.HandleFunc("/_debug/state", s.handleState)
	s.mux.HandleFunc("/_debug/faults", s.handleFaults)
	s.mux.HandleFunc("/_debug/faults/reset", s.handleFaultsReset)
	s.mux.HandleFunc("/_debug/config", s.handleConfig)
	s.mux.HandleFunc("/_debug/identity", s.handleIdentity)
}

// handleIndex serves the debug interface index page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/_debug/" {
		http.NotFound(w, r)
		return
	}

	const html = `<!DOCTYPE html>
<html>
<head><title>SPIRE Debug</title></head>
<body>
<h1>SPIRE Identity Library - Debug Interface</h1>
<p><strong>⚠️ WARNING:</strong> This is a debug interface. Never use in production.</p>
<h2>Available Endpoints:</h2>
<ul>
<li><a href="/_debug/state">/_debug/state</a> - View current state</li>
<li><a href="/_debug/identity">/_debug/identity</a> - View identity snapshot (certs, auth decisions)</li>
<li><a href="/_debug/faults">/_debug/faults</a> - View/modify fault injection (GET/POST)</li>
<li><a href="/_debug/faults/reset">/_debug/faults/reset</a> - Reset all faults (POST)</li>
<li><a href="/_debug/config">/_debug/config</a> - View debug configuration</li>
</ul>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// handleState exposes high-level debug runtime toggles and current fault state.
// This endpoint intentionally does NOT include secrets or identity material.
// Safe to serve in both "debug" and "staging" modes. Still loopback-only.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	state := map[string]any{
		"debug_enabled": Active.Enabled,
		"mode":          Active.Mode, // "debug", "staging", or "production"
		"stress_mode":   Active.Stress,
		"single_thread": Active.SingleThreaded,
		"faults":        Faults.Snapshot(),
	}

	// NOTE: All debug JSON MUST be written via writeJSON / writeJSONStatus.
	// These helpers set Cache-Control: no-store and Content-Type correctly.
	// Do not inline json.NewEncoder(...).Encode(...) here.
	writeJSON(w, state)
}

// handleFaults handles GET and POST requests for fault injection.
// POST is only allowed in "debug" mode, not "staging" or "production".
func (s *Server) handleFaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getFaults(w, r)
	case http.MethodPost:
		// Only allow mutation if we're explicitly in "debug" mode, not "staging".
		if Active.Mode != "debug" {
			http.Error(w, "Fault injection disabled in this mode", http.StatusForbidden)
			return
		}
		s.setFaults(w, r)
	default:
		methodNotAllowed(w)
	}
}

// getFaults returns the current fault configuration.
func (s *Server) getFaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, Faults.Snapshot())
}

// setFaults applies fault injection configuration from JSON request.
func (s *Server) setFaults(w http.ResponseWriter, r *http.Request) {
	logger := GetLogger()

	// Limit request body size to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req FaultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Debugf("Failed to decode fault request: %v", err)
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Apply faults using type-safe struct fields
	if req.DropNextHandshake != nil {
		Faults.SetDropNextHandshake(*req.DropNextHandshake)
		logger.Debugf("Fault set: drop_next_handshake=%v", *req.DropNextHandshake)
	}

	if req.CorruptNextSPIFFEID != nil {
		Faults.SetCorruptNextSPIFFEID(*req.CorruptNextSPIFFEID)
		logger.Debugf("Fault set: corrupt_next_spiffe_id=%v", *req.CorruptNextSPIFFEID)
	}

	if req.DelayNextIssueSeconds != nil {
		if err := Faults.SetDelayNextIssue(*req.DelayNextIssueSeconds); err != nil {
			logger.Debugf("Invalid delay value: %v", err)
			http.Error(w, fmt.Sprintf("Invalid delay: %v", err), http.StatusBadRequest)
			return
		}
		logger.Debugf("Fault set: delay_next_issue_seconds=%d", *req.DelayNextIssueSeconds)
	}

	if req.ForceTrustDomainMismatch != nil {
		Faults.SetForceTrustDomainMismatch(*req.ForceTrustDomainMismatch)
		logger.Debugf("Fault set: force_trust_domain_mismatch=%v", *req.ForceTrustDomainMismatch)
	}

	if req.ForceExpiredCert != nil {
		Faults.SetForceExpiredCert(*req.ForceExpiredCert)
		logger.Debugf("Fault set: force_expired_cert=%v", *req.ForceExpiredCert)
	}

	if req.RejectNextWorkloadLookup != nil {
		Faults.SetRejectNextWorkloadLookup(*req.RejectNextWorkloadLookup)
		logger.Debugf("Fault set: reject_next_workload_lookup=%v", *req.RejectNextWorkloadLookup)
	}

	// Return current state
	s.getFaults(w, r)
}

// handleFaultsReset resets all fault injections.
// Only allowed in "debug" mode, not "staging" or "production".
func (s *Server) handleFaultsReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	// Only allow mutation if we're explicitly in "debug" mode, not "staging".
	if Active.Mode != "debug" {
		http.Error(w, "Fault injection disabled in this mode", http.StatusForbidden)
		return
	}

	Faults.Reset()
	GetLogger().Debug("All faults reset")

	writeJSON(w, map[string]string{"status": "reset"})
}

// handleConfig returns the current debug configuration.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	config := map[string]any{
		"enabled":            Active.Enabled,
		"mode":               Active.Mode,
		"stress":             Active.Stress,
		"single_threaded":    Active.SingleThreaded,
		"local_debug_server": Active.LocalDebugServer,
		"debug_server_addr":  Active.DebugServerAddr,
	}

	writeJSON(w, config)
}

// handleIdentity returns a snapshot of the current identity state.
// This endpoint is only available if an introspector was provided to Start().
// Returns 503 Service Unavailable if identity state has errors.
func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	if s.introspector == nil {
		http.Error(w, "Identity introspection not available (no introspector provided)", http.StatusNotImplemented)
		return
	}

	snapshot := s.introspector.SnapshotData(r.Context())

	// Return 503 if any errors in identity state
	status := http.StatusOK
	for _, d := range snapshot.RecentDecisions {
		if d.Decision == "ERROR" {
			status = http.StatusServiceUnavailable
			break
		}
	}

	// NOTE: All debug JSON MUST be written via writeJSON / writeJSONStatus.
	// These helpers enforce the no-store header. See writeJSONStatus docs/tests.
	writeJSONStatus(w, status, snapshot)
}

// writeJSONStatus writes a JSON response with the given status code.
// Security contract:
//   • Sets "Cache-Control: no-store" on ALL debug JSON responses to prevent
//     intermediaries, CLIs, or proxies from caching or logging operational state
//     (trust domains, SPIFFE IDs, fault toggles, etc.).
//   • Sets Content-Type to application/json; charset=utf-8.
//   • All debug endpoints MUST use this helper (or writeJSON) to avoid drift.
// Any change to these headers MUST update TestServer_handleIdentity_NoStoreHeader.
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	// Prevent caching of potentially sensitive debug state
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSON writes a JSON response with 200 OK status and proper content type.
func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

// methodNotAllowed writes a 405 Method Not Allowed response.
// This intentionally does NOT use writeJSONStatus because it returns no sensitive
// runtime state (just a static error message). All endpoints that return operational
// data (state, identity, faults, etc.) MUST use writeJSON/writeJSONStatus.
func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Stop shuts down the debug server gracefully.
// Returns nil if the server was not started or is already stopped.
// The context controls the shutdown timeout.
// This method is idempotent - safe to call multiple times.
func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}

	GetLogger().Debug("Shutting down debug server")
	err := s.httpServer.Shutdown(ctx)

	// Make subsequent Stop() calls a no-op
	s.httpServer = nil

	return err
}

// isLoopback returns true if addr is a loopback-only listen address.
// Allowed forms:
//   - "127.x.y.z:port" (any 127.0.0.0/8 IPv4)
//   - "[::1]:port"     (IPv6 loopback)
//   - "localhost:port" (literal hostname only)
//
// Security contract:
//   • addr MUST include a port. Bare hosts like "127.0.0.1" are rejected
//     (SplitHostPort fails) and MUST remain rejected. This prevents someone
//     from "helpfully" accepting broader forms like "0.0.0.0" without port.
//   • Arbitrary hostnames are NOT allowed. Only literal "localhost" is
//     allowed. "api.example.com:6060" MUST be rejected even if it currently
//     resolves to 127.0.0.1 at runtime.
//   • Returning true here is what allows Start() to proceed and later log the
//     bound address. That log line is considered safe *only because* of this
//     check. Weakening this function without updating that log is a data leak.
//
// Any change to this logic MUST update tests that cover Start()/isLoopback.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}

	// Allow literal "localhost" to reduce footguns in dev configs.
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
