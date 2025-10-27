//go:build debug

package debug

import (
	"encoding/json"
	"fmt"
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
func Start(introspector Introspector) {
	if !Active.LocalDebugServer {
		return
	}

	srv := &Server{
		addr:         Active.DebugServerAddr,
		mux:          http.NewServeMux(),
		introspector: introspector,
	}
	srv.registerHandlers()

	go func() {
		logger := GetLogger()
		logger.Debugf("⚠️  DEBUG SERVER RUNNING ON %s", srv.addr)
		logger.Debug("⚠️  WARNING: Debug mode is enabled. DO NOT USE IN PRODUCTION!")

		// Use http.Server for better control and basic hardening
		httpServer := &http.Server{
			Addr:              srv.addr,
			Handler:           srv.mux,
			ReadHeaderTimeout: 2 * time.Second, // Prevent Slowloris attacks
		}

		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Debugf("Debug server error: %v", err)
		}
	}()
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

// handleState serves the current debug state.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	state := map[string]any{
		"debug_enabled": Active.Enabled,
		"stress_mode":   Active.Stress,
		"single_thread": Active.SingleThreaded,
		"faults":        Faults.Snapshot(),
	}

	writeJSON(w, state)
}

// handleFaults handles GET and POST requests for fault injection.
func (s *Server) handleFaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getFaults(w, r)
	case http.MethodPost:
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
func (s *Server) handleFaultsReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	Faults.Reset()
	GetLogger().Debug("All faults reset")

	writeJSON(w, map[string]string{"status": "reset"})
}

// handleConfig returns the current debug configuration.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	config := map[string]any{
		"enabled":            Active.Enabled,
		"stress":             Active.Stress,
		"single_threaded":    Active.SingleThreaded,
		"local_debug_server": Active.LocalDebugServer,
		"debug_server_addr":  Active.DebugServerAddr,
	}

	writeJSON(w, config)
}

// handleIdentity returns a snapshot of the current identity state.
// This endpoint is only available if an introspector was provided to Start().
func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	if s.introspector == nil {
		http.Error(w, "Identity introspection not available (no introspector provided)", http.StatusNotImplemented)
		return
	}

	snapshot := s.introspector.SnapshotData(r.Context())
	writeJSON(w, snapshot)
}

// writeJSON writes a JSON response with proper content type.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// methodNotAllowed writes a 405 Method Not Allowed response.
func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}
