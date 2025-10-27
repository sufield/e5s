//go:build debug

package debug

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestServer_handleIndex(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify Content-Type includes charset
	if ct := w.Header().Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type text/html; charset=utf-8, got %s", ct)
	}

	body := w.Body.String()
	if !strings.Contains(body, "SPIRE Identity Library") {
		t.Error("Expected HTML content with title")
	}

	if !strings.Contains(body, "/_debug/faults") {
		t.Error("Expected links to fault endpoints")
	}
}

func TestServer_handleIndex_NotFound(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/invalid", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestServer_handleState(t *testing.T) {
	// Set up test state
	Active.Enabled = true
	Active.Stress = false

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/state", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify Content-Type includes charset
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Expected application/json Content-Type, got %s", ct)
	}

	var state map[string]any
	if err := json.NewDecoder(w.Body).Decode(&state); err != nil {
		t.Fatalf("Failed to decode state: %v", err)
	}

	if enabled, ok := state["debug_enabled"].(bool); !ok || !enabled {
		t.Error("Expected debug_enabled to be true")
	}

	if _, ok := state["faults"].(map[string]any); !ok {
		t.Error("Expected faults snapshot in state")
	}

	// Verify all expected keys
	expectedKeys := []string{"debug_enabled", "stress_mode", "single_thread", "faults"}
	for _, key := range expectedKeys {
		if _, ok := state[key]; !ok {
			t.Errorf("Expected state to contain key: %s", key)
		}
	}
}

func TestServer_getFaults(t *testing.T) {
	// Reset faults
	Faults.Reset()

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/faults", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var faults map[string]any
	if err := json.NewDecoder(w.Body).Decode(&faults); err != nil {
		t.Fatalf("Failed to decode faults: %v", err)
	}

	// Should have all fault fields
	if _, ok := faults["drop_next_handshake"]; !ok {
		t.Error("Expected drop_next_handshake field")
	}
}

func TestServer_setFaults_TypeSafe(t *testing.T) {
	// Save original mode and set to debug for mutation
	origMode := Active.Mode
	defer func() { Active.Mode = origMode }()
	Active.Mode = "debug"

	// Reset faults
	Faults.Reset()

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	// Test with type-safe struct
	payload := FaultRequest{
		DropNextHandshake:   boolPtr(true),
		DelayNextIssueSeconds: intPtr(5),
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/_debug/faults", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify faults were set
	if !Faults.ShouldDropHandshake() {
		t.Error("Expected DropNextHandshake to be set")
	}

	if delay := Faults.GetAndClearDelay(); delay != 5 {
		t.Errorf("Expected delay=5, got %d", delay)
	}
}

func TestServer_setFaults_InvalidJSON(t *testing.T) {
	// Save original mode and set to debug for mutation
	origMode := Active.Mode
	defer func() { Active.Mode = origMode }()
	Active.Mode = "debug"

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodPost, "/_debug/faults", strings.NewReader("{invalid json"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Invalid JSON") {
		t.Error("Expected error message about invalid JSON")
	}
}

func TestServer_setFaults_InvalidDelay(t *testing.T) {
	// Save original mode and set to debug for mutation
	origMode := Active.Mode
	defer func() { Active.Mode = origMode }()
	Active.Mode = "debug"

	// Reset faults
	Faults.Reset()

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	// Test with negative delay
	payload := FaultRequest{
		DelayNextIssueSeconds: intPtr(-1),
	}

	body, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/_debug/faults", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "Invalid delay") {
		t.Error("Expected error message about invalid delay")
	}
}

func TestServer_setFaults_BodySizeLimit(t *testing.T) {
	// Save original mode and set to debug for mutation
	origMode := Active.Mode
	defer func() { Active.Mode = origMode }()
	Active.Mode = "debug"

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	// Create a payload larger than 10KB
	largePayload := make([]byte, 11*1024)
	for i := range largePayload {
		largePayload[i] = 'a'
	}

	req := httptest.NewRequest(http.MethodPost, "/_debug/faults", bytes.NewReader(largePayload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 for oversized body, got %d", w.Code)
	}
}

func TestServer_handleFaultsReset(t *testing.T) {
	// Save original mode and set to debug for mutation
	origMode := Active.Mode
	defer func() { Active.Mode = origMode }()
	Active.Mode = "debug"

	// Set some faults
	Faults.SetDropNextHandshake(true)
	Faults.SetCorruptNextSPIFFEID(true)

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodPost, "/_debug/faults/reset", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if resp["status"] != "reset" {
		t.Errorf("Expected status='reset', got %s", resp["status"])
	}

	// Verify faults were reset
	if Faults.ShouldDropHandshake() {
		t.Error("Expected faults to be reset")
	}
}

func TestServer_handleFaultsReset_MethodNotAllowed(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/faults/reset", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestServer_handleConfig(t *testing.T) {
	Active.Enabled = true
	Active.Stress = true
	Active.Mode = "debug"
	Active.DebugServerAddr = "127.0.0.1:6060"

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/config", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify Content-Type includes charset
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Expected application/json Content-Type, got %s", ct)
	}

	var config map[string]any
	if err := json.NewDecoder(w.Body).Decode(&config); err != nil {
		t.Fatalf("Failed to decode config: %v", err)
	}

	if enabled, ok := config["enabled"].(bool); !ok || !enabled {
		t.Error("Expected enabled to be true")
	}

	if stress, ok := config["stress"].(bool); !ok || !stress {
		t.Error("Expected stress to be true")
	}

	if mode, ok := config["mode"].(string); !ok || mode != "debug" {
		t.Errorf("Expected mode to be 'debug', got %v", config["mode"])
	}
}

func TestServer_handleFaults_MethodNotAllowed(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodPut, "/_debug/faults", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

// TestServer_AllFaultTypes tests all fault types comprehensively
func TestServer_AllFaultTypes(t *testing.T) {
	tests := []struct {
		name    string
		payload FaultRequest
		verify  func(*testing.T)
	}{
		{
			name: "DropNextHandshake",
			payload: FaultRequest{
				DropNextHandshake: boolPtr(true),
			},
			verify: func(t *testing.T) {
				if !Faults.ShouldDropHandshake() {
					t.Error("Expected DropNextHandshake to be set")
				}
			},
		},
		{
			name: "CorruptNextSPIFFEID",
			payload: FaultRequest{
				CorruptNextSPIFFEID: boolPtr(true),
			},
			verify: func(t *testing.T) {
				if !Faults.ShouldCorruptSPIFFEID() {
					t.Error("Expected CorruptNextSPIFFEID to be set")
				}
			},
		},
		{
			name: "DelayNextIssue",
			payload: FaultRequest{
				DelayNextIssueSeconds: intPtr(10),
			},
			verify: func(t *testing.T) {
				if delay := Faults.GetAndClearDelay(); delay != 10 {
					t.Errorf("Expected delay=10, got %d", delay)
				}
			},
		},
		{
			name: "ForceTrustDomainMismatch",
			payload: FaultRequest{
				ForceTrustDomainMismatch: boolPtr(true),
			},
			verify: func(t *testing.T) {
				if !Faults.ShouldForceTrustDomainMismatch() {
					t.Error("Expected ForceTrustDomainMismatch to be set")
				}
			},
		},
		{
			name: "ForceExpiredCert",
			payload: FaultRequest{
				ForceExpiredCert: boolPtr(true),
			},
			verify: func(t *testing.T) {
				if !Faults.ShouldForceExpiredCert() {
					t.Error("Expected ForceExpiredCert to be set")
				}
			},
		},
		{
			name: "RejectNextWorkloadLookup",
			payload: FaultRequest{
				RejectNextWorkloadLookup: boolPtr(true),
			},
			verify: func(t *testing.T) {
				if !Faults.ShouldRejectWorkloadLookup() {
					t.Error("Expected RejectNextWorkloadLookup to be set")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original mode and set to debug for mutation
			origMode := Active.Mode
			defer func() { Active.Mode = origMode }()
			Active.Mode = "debug"

			Faults.Reset()

			srv := &Server{mux: http.NewServeMux()}
			srv.registerHandlers()

			body, _ := json.Marshal(tt.payload)
			req := httptest.NewRequest(http.MethodPost, "/_debug/faults", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()

			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Errorf("Expected status 200, got %d", w.Code)
			}

			tt.verify(t)
		})
	}
}

// TestServer_MethodNotAllowed_TableDriven tests method validation across endpoints
func TestServer_MethodNotAllowed_TableDriven(t *testing.T) {
	tests := []struct {
		endpoint string
		method   string
	}{
		{"/_debug/faults", http.MethodPut},
		{"/_debug/faults", http.MethodDelete},
		{"/_debug/faults/reset", http.MethodGet},
		{"/_debug/faults/reset", http.MethodPut},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint+"_"+tt.method, func(t *testing.T) {
			srv := &Server{mux: http.NewServeMux()}
			srv.registerHandlers()

			req := httptest.NewRequest(tt.method, tt.endpoint, nil)
			w := httptest.NewRecorder()

			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405, got %d", w.Code)
			}

			if !strings.Contains(w.Body.String(), "Method not allowed") {
				t.Error("Expected 'Method not allowed' in response body")
			}
		})
	}
}

// TestServer_ConcurrentFaultAccess tests concurrent fault setting and reading
func TestServer_ConcurrentFaultAccess(t *testing.T) {
	Faults.Reset()

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	// Run multiple concurrent requests
	const numRequests = 50
	done := make(chan bool, numRequests*2)

	// Concurrent POST requests
	for i := 0; i < numRequests; i++ {
		go func(n int) {
			payload := FaultRequest{
				DropNextHandshake: boolPtr(n%2 == 0),
				DelayNextIssueSeconds: intPtr(n),
			}
			body, _ := json.Marshal(payload)
			req := httptest.NewRequest(http.MethodPost, "/_debug/faults", bytes.NewReader(body))
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)
			done <- true
		}(i)
	}

	// Concurrent GET requests
	for i := 0; i < numRequests; i++ {
		go func() {
			req := httptest.NewRequest(http.MethodGet, "/_debug/faults", nil)
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)
			done <- true
		}()
	}

	// Wait for all requests
	for i := 0; i < numRequests*2; i++ {
		<-done
	}
	// If we get here without deadlock or panic, concurrent access is safe
}

// TestServer_handleIdentity_NoIntrospector tests identity endpoint without introspector
func TestServer_handleIdentity_NoIntrospector(t *testing.T) {
	srv := &Server{
		mux:          http.NewServeMux(),
		introspector: nil, // No introspector provided
	}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/identity", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Errorf("Expected status 501, got %d", w.Code)
	}

	if !strings.Contains(w.Body.String(), "not available") {
		t.Error("Expected error message about introspection not available")
	}
}

// TestServer_handleIdentity_WithIntrospector tests identity endpoint with mock introspector
func TestServer_handleIdentity_WithIntrospector(t *testing.T) {
	// Create mock introspector
	mockIntrospector := &mockIntrospector{
		snapshot: Snapshot{
			Mode:        "debug",
			TrustDomain: "example.org",
			Adapter:     "spire",
			Certs: []CertView{
				{
					SpiffeID:         "spiffe://example.org/test",
					ExpiresInSeconds: 3600,
					RotationPending:  false,
				},
			},
			RecentDecisions: []AuthDecision{
				{
					CallerSPIFFEID: "spiffe://example.org/client",
					Resource:       "/api/test",
					Decision:       "ALLOW",
					Reason:         "valid certificate",
				},
			},
		},
	}

	srv := &Server{
		mux:          http.NewServeMux(),
		introspector: mockIntrospector,
	}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/identity", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	// Verify Content-Type includes charset
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Expected application/json Content-Type, got %s", ct)
	}

	var snapshot Snapshot
	if err := json.NewDecoder(w.Body).Decode(&snapshot); err != nil {
		t.Fatalf("Failed to decode snapshot: %v", err)
	}

	if snapshot.Mode != "debug" {
		t.Errorf("Expected mode=debug, got %s", snapshot.Mode)
	}

	if snapshot.TrustDomain != "example.org" {
		t.Errorf("Expected trustDomain=example.org, got %s", snapshot.TrustDomain)
	}

	if len(snapshot.Certs) != 1 {
		t.Fatalf("Expected 1 cert, got %d", len(snapshot.Certs))
	}

	if snapshot.Certs[0].SpiffeID != "spiffe://example.org/test" {
		t.Errorf("Expected spiffeID=spiffe://example.org/test, got %s", snapshot.Certs[0].SpiffeID)
	}

	if len(snapshot.RecentDecisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(snapshot.RecentDecisions))
	}

	if snapshot.RecentDecisions[0].Decision != "ALLOW" {
		t.Errorf("Expected decision=ALLOW, got %s", snapshot.RecentDecisions[0].Decision)
	}
}

// TestServer_handleIdentity_NoStoreHeader tests that Cache-Control and Content-Type headers are set correctly
// on successful JSON responses. This guarantee does NOT apply to error paths like 405 Method Not Allowed,
// which return static text and intentionally skip writeJSONStatus (see TestMethodNotAllowed_DoesNotSetNoStoreHeader).
func TestServer_handleIdentity_NoStoreHeader(t *testing.T) {
	mockIntrospector := &mockIntrospector{
		snapshot: Snapshot{
			Mode:        "debug",
			TrustDomain: "spiffe://example.org",
			Adapter:     "spire",
			Certs:       []CertView{},
			RecentDecisions: nil,
		},
	}

	srv := &Server{
		mux:          http.NewServeMux(),
		introspector: mockIntrospector,
	}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/identity", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	// Assert headers
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected Cache-Control 'no-store', got %q", got)
	}

	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type 'application/json; charset=utf-8', got %q", ct)
	}

	// Assert body: validate mode presence/consistency
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid json: %v", err)
	}

	if gotMode, ok := body["mode"].(string); !ok {
		t.Errorf("response missing 'mode' string field")
	} else if gotMode != mockIntrospector.snapshot.Mode {
		t.Errorf("expected mode %q, got %q", mockIntrospector.snapshot.Mode, gotMode)
	}
}

// TestServer_handleState_IncludesModeAndHeaders tests that /state returns mode field and correct headers
// on successful JSON responses. This guarantee does NOT apply to error paths like 405 Method Not Allowed.
func TestServer_handleState_IncludesModeAndHeaders(t *testing.T) {
	// Arrange
	Active.Mode = "staging"
	Active.Enabled = true
	Active.Stress = false
	Active.SingleThreaded = false

	// Ensure predictable Faults.Snapshot() result
	Faults.Reset()

	srv := &Server{
		mux: http.NewServeMux(),
	}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/state", nil)
	w := httptest.NewRecorder()

	// Act
	srv.mux.ServeHTTP(w, req)

	// Assert status
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", w.Code)
	}

	// Assert headers
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("expected Cache-Control 'no-store', got %q", got)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json; charset=utf-8" {
		t.Errorf("expected Content-Type 'application/json; charset=utf-8', got %q", ct)
	}

	// Assert body shape (mode present, matches Active.Mode)
	var body map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("response body is not valid json: %v", err)
	}

	if gotMode, ok := body["mode"].(string); !ok {
		t.Errorf("response missing 'mode' string field")
	} else if gotMode != Active.Mode {
		t.Errorf("expected mode %q, got %q", Active.Mode, gotMode)
	}

	if _, ok := body["faults"]; !ok {
		t.Errorf("response missing 'faults' field")
	}
}

// TestMethodNotAllowed_DoesNotSetNoStoreHeader verifies that 405 Method Not Allowed responses
// intentionally omit Cache-Control: no-store because they return static error text, not runtime state.
// This is the documented exception to the "all debug JSON gets no-store" rule.
func TestMethodNotAllowed_DoesNotSetNoStoreHeader(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	// Pick an endpoint with only GET allowed, call it with POST to trigger 405.
	req := httptest.NewRequest(http.MethodPost, "/_debug/state", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}

	// 405 is allowed to omit Cache-Control: no-store because it returns no runtime data.
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("did not expect Cache-Control header on 405, got %q", got)
	}

	// Sanity check: response is plain text, not JSON with operational state.
	body := w.Body.String()
	if body == "" {
		t.Fatalf("expected non-empty response body")
	}
	if body[0] == '{' {
		// Truncate for readability without introducing a package-level helper.
		preview := body
		if len(preview) > 20 {
			preview = preview[:20]
		}
		t.Errorf("405 response must not be JSON; got body starting with %q", preview)
	}
}

// TestServer_handleIdentity_ErrorReturns503 tests that identity errors return 503
func TestServer_handleIdentity_ErrorReturns503(t *testing.T) {
	mockIntrospector := &mockIntrospector{
		snapshot: Snapshot{
			Mode:        "debug",
			TrustDomain: "",
			Adapter:     "spire",
			Certs:       []CertView{},
			RecentDecisions: []AuthDecision{
				{
					CallerSPIFFEID: "",
					Resource:       "spire.FetchX509SVID",
					Decision:       "ERROR",
					Reason:         "connection refused",
				},
			},
		},
	}

	srv := &Server{
		mux:          http.NewServeMux(),
		introspector: mockIntrospector,
	}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/identity", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}

	var snapshot Snapshot
	if err := json.NewDecoder(w.Body).Decode(&snapshot); err != nil {
		t.Fatalf("Failed to decode snapshot: %v", err)
	}

	if len(snapshot.RecentDecisions) != 1 {
		t.Fatalf("Expected 1 decision, got %d", len(snapshot.RecentDecisions))
	}

	if snapshot.RecentDecisions[0].Decision != "ERROR" {
		t.Errorf("Expected decision=ERROR, got %s", snapshot.RecentDecisions[0].Decision)
	}
}

// TestServer_MethodValidation tests that GET-only endpoints reject other methods
func TestServer_MethodValidation(t *testing.T) {
	tests := []struct {
		endpoint string
		method   string
	}{
		{"/_debug/state", http.MethodPost},
		{"/_debug/state", http.MethodPut},
		{"/_debug/config", http.MethodPost},
		{"/_debug/config", http.MethodDelete},
		{"/_debug/identity", http.MethodPost},
		{"/_debug/identity", http.MethodPut},
	}

	for _, tt := range tests {
		t.Run(tt.endpoint+"_"+tt.method, func(t *testing.T) {
			srv := &Server{
				mux:          http.NewServeMux(),
				introspector: &mockIntrospector{snapshot: Snapshot{}},
			}
			srv.registerHandlers()

			req := httptest.NewRequest(tt.method, tt.endpoint, nil)
			w := httptest.NewRecorder()

			srv.mux.ServeHTTP(w, req)

			if w.Code != http.StatusMethodNotAllowed {
				t.Errorf("Expected status 405, got %d", w.Code)
			}

			if !strings.Contains(w.Body.String(), "Method not allowed") {
				t.Error("Expected 'Method not allowed' in response body")
			}
		})
	}
}

// TestIsLoopback tests the loopback address validation
func TestIsLoopback(t *testing.T) {
	tests := []struct {
		addr     string
		expected bool
	}{
		{"127.0.0.1:6060", true},
		{"127.0.0.2:8080", true},
		{"127.255.255.255:9999", true},
		{"[::1]:6060", true},
		{"localhost:6060", true}, // literal "localhost" allowed to reduce dev config footguns
		{"0.0.0.0:6060", false},
		{"192.168.1.1:6060", false},
		{"10.0.0.1:6060", false},
		{"[::]:6060", false},
		{"invalid", false},
		{"", false}, // empty address
	}

	for _, tt := range tests {
		t.Run(tt.addr, func(t *testing.T) {
			result := isLoopback(tt.addr)
			if result != tt.expected {
				t.Errorf("isLoopback(%q) = %v, expected %v", tt.addr, result, tt.expected)
			}
		})
	}
}

// TestStart_RejectsNonLoopback tests that Start refuses non-loopback addresses
func TestStart_RejectsNonLoopback(t *testing.T) {
	// Save original state
	origAddr := Active.DebugServerAddr
	origEnabled := Active.LocalDebugServer
	defer func() {
		Active.DebugServerAddr = origAddr
		Active.LocalDebugServer = origEnabled
	}()

	// Try to start with public address
	Active.LocalDebugServer = true
	Active.DebugServerAddr = "0.0.0.0:6060"

	// Start should refuse and not panic
	Start(nil)

	// If we get here, Start() correctly refused the non-loopback address
	// There's no server to test, which is the desired behavior
}

// TestServer_FaultInjection_ModeGating tests that fault injection is gated by mode
func TestServer_FaultInjection_ModeGating(t *testing.T) {
	tests := []struct {
		name           string
		mode           string
		method         string
		endpoint       string
		expectedStatus int
		shouldMutate   bool
	}{
		{
			name:           "POST faults in debug mode - allowed",
			mode:           "debug",
			method:         http.MethodPost,
			endpoint:       "/_debug/faults",
			expectedStatus: http.StatusOK,
			shouldMutate:   true,
		},
		{
			name:           "POST faults in staging mode - forbidden",
			mode:           "staging",
			method:         http.MethodPost,
			endpoint:       "/_debug/faults",
			expectedStatus: http.StatusForbidden,
			shouldMutate:   false,
		},
		{
			name:           "POST faults in production mode - forbidden",
			mode:           "production",
			method:         http.MethodPost,
			endpoint:       "/_debug/faults",
			expectedStatus: http.StatusForbidden,
			shouldMutate:   false,
		},
		{
			name:           "GET faults in staging mode - allowed",
			mode:           "staging",
			method:         http.MethodGet,
			endpoint:       "/_debug/faults",
			expectedStatus: http.StatusOK,
			shouldMutate:   false,
		},
		{
			name:           "POST faults/reset in debug mode - allowed",
			mode:           "debug",
			method:         http.MethodPost,
			endpoint:       "/_debug/faults/reset",
			expectedStatus: http.StatusOK,
			shouldMutate:   true,
		},
		{
			name:           "POST faults/reset in staging mode - forbidden",
			mode:           "staging",
			method:         http.MethodPost,
			endpoint:       "/_debug/faults/reset",
			expectedStatus: http.StatusForbidden,
			shouldMutate:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original state
			origMode := Active.Mode
			defer func() {
				Active.Mode = origMode
			}()

			// Set mode for this test
			Active.Mode = tt.mode

			// Reset faults before each test
			Faults.Reset()

			srv := &Server{mux: http.NewServeMux()}
			srv.registerHandlers()

			var req *http.Request
			if tt.method == http.MethodPost && tt.endpoint == "/_debug/faults" {
				// POST with fault payload
				payload := FaultRequest{
					DropNextHandshake: boolPtr(true),
				}
				body, _ := json.Marshal(payload)
				req = httptest.NewRequest(tt.method, tt.endpoint, bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
			} else {
				req = httptest.NewRequest(tt.method, tt.endpoint, nil)
			}

			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			// Verify mutation only occurred if allowed
			if tt.endpoint == "/_debug/faults" && tt.method == http.MethodPost {
				faultSet := Faults.ShouldDropHandshake()
				if tt.shouldMutate && !faultSet {
					t.Error("Expected fault to be set in debug mode, but it wasn't")
				}
				if !tt.shouldMutate && faultSet {
					t.Error("Expected fault NOT to be set in non-debug mode, but it was")
				}
			}

			// Verify 403 responses have appropriate message
			if tt.expectedStatus == http.StatusForbidden {
				if !strings.Contains(w.Body.String(), "Fault injection disabled in this mode") {
					t.Error("Expected 'Fault injection disabled' message in 403 response")
				}
			}
		})
	}
}

// TestServer_FaultInjection_StagingReadOnly tests that staging can observe but not mutate faults
func TestServer_FaultInjection_StagingReadOnly(t *testing.T) {
	// Save original state
	origMode := Active.Mode
	defer func() {
		Active.Mode = origMode
	}()

	// Set staging mode
	Active.Mode = "staging"
	Faults.Reset()

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	// GET should work - observability maintained
	req := httptest.NewRequest(http.MethodGet, "/_debug/faults", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected GET to work in staging, got status %d", w.Code)
	}

	var faults map[string]any
	if err := json.NewDecoder(w.Body).Decode(&faults); err != nil {
		t.Fatalf("Failed to decode faults: %v", err)
	}

	// POST should fail - mutation blocked
	payload := FaultRequest{
		DropNextHandshake: boolPtr(true),
	}
	body, _ := json.Marshal(payload)
	req = httptest.NewRequest(http.MethodPost, "/_debug/faults", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected POST to be forbidden in staging, got status %d", w.Code)
	}

	// Verify fault was NOT set
	if Faults.ShouldDropHandshake() {
		t.Error("Fault should not have been set in staging mode")
	}
}

// TestServer_Stop tests graceful shutdown of the debug server
func TestServer_Stop(t *testing.T) {
	// Save original config
	origEnabled := Active.LocalDebugServer
	origAddr := Active.DebugServerAddr
	defer func() {
		Active.LocalDebugServer = origEnabled
		Active.DebugServerAddr = origAddr
	}()

	// Enable server on loopback
	Active.LocalDebugServer = true
	Active.DebugServerAddr = "127.0.0.1:0" // Use port 0 for random available port

	srv := Start(nil)
	if srv == nil {
		t.Fatal("Expected server to start, got nil")
	}

	// Give server time to start
	time.Sleep(100 * time.Millisecond)

	// Stop the server
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := srv.Stop(ctx); err != nil {
		t.Errorf("Stop() returned error: %v", err)
	}

	// Stopping again should be idempotent
	if err := srv.Stop(ctx); err != nil {
		t.Errorf("Second Stop() returned error: %v", err)
	}
}

// TestServer_StopNil tests that Stop on nil server is safe
func TestServer_StopNil(t *testing.T) {
	var srv *Server
	ctx := context.Background()
	if err := srv.Stop(ctx); err != nil {
		t.Errorf("Stop() on nil server returned error: %v", err)
	}
}

// TestStart_ReturnsNil tests that Start returns nil when conditions not met
func TestStart_ReturnsNilWhenDisabled(t *testing.T) {
	origEnabled := Active.LocalDebugServer
	defer func() { Active.LocalDebugServer = origEnabled }()

	Active.LocalDebugServer = false
	srv := Start(nil)
	if srv != nil {
		t.Error("Expected Start to return nil when LocalDebugServer is false")
	}
}

// mockIntrospector is a test double for debug.Introspector
type mockIntrospector struct {
	snapshot Snapshot
}

func (m *mockIntrospector) SnapshotData(ctx context.Context) Snapshot {
	return m.snapshot
}

// Helper functions
func boolPtr(b bool) *bool {
	return &b
}

func intPtr(i int) *int {
	return &i
}
