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

	// Verify Content-Type
	if ct := w.Header().Get("Content-Type"); ct != "text/html" {
		t.Errorf("Expected Content-Type text/html, got %s", ct)
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

	// Verify Content-Type
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", ct)
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
	Active.DebugServerAddr = "127.0.0.1:6060"

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/config", nil)
	w := httptest.NewRecorder()

	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
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
