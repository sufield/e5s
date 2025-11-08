package e5s_test

import (
	"net/http"
	"testing"

	"github.com/sufield/e5s"
)

// TestStartSingleThread_InvalidConfig verifies StartSingleThread fails with invalid configuration.
// Note: We only test failure cases because StartSingleThread blocks indefinitely on success
// and cannot be easily stopped without external signals. Success cases are covered by
// integration tests that use Start() with shutdown control.
func TestStartSingleThread_InvalidConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Test with non-existent config file
	err := e5s.StartSingleThread("/nonexistent/config.yaml", handler)
	if err == nil {
		t.Fatal("expected error with non-existent config file, got nil")
	}
}
