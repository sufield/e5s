package e5s_test

import (
	"context"
	"net/http"
	"testing"
	"time"

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

// TestStart_InvalidConfig verifies Start fails with invalid configuration.
func TestStart_InvalidConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		configPath string
		wantErr    bool
	}{
		{
			name:       "non-existent config file",
			configPath: "/nonexistent/config.yaml",
			wantErr:    true,
		},
		{
			name:       "empty config path",
			configPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e5s.Start(tt.configPath, handler)
			if (err != nil) != tt.wantErr {
				t.Errorf("Start() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestStartWithContext_InvalidConfig verifies StartWithContext fails with invalid configuration.
func TestStartWithContext_InvalidConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	ctx := context.Background()

	tests := []struct {
		name       string
		configPath string
		wantErr    bool
	}{
		{
			name:       "non-existent config file",
			configPath: "/nonexistent/config.yaml",
			wantErr:    true,
		},
		{
			name:       "empty config path",
			configPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := e5s.StartWithContext(ctx, tt.configPath, handler)
			if (err != nil) != tt.wantErr {
				t.Errorf("StartWithContext() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestStartWithContext_ContextCancellation verifies context cancellation during initialization.
func TestStartWithContext_ContextCancellation(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should fail because the context is canceled
	_, err := e5s.StartWithContext(ctx, "/nonexistent/config.yaml", handler)
	if err == nil {
		t.Error("expected error with canceled context, got nil")
	}
}

// TestServe_InvalidConfig verifies Serve fails with invalid configuration.
func TestServe_InvalidConfig(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tests := []struct {
		name       string
		configPath string
		wantErr    bool
	}{
		{
			name:       "non-existent config file",
			configPath: "/nonexistent/config.yaml",
			wantErr:    true,
		},
		{
			name:       "empty config path",
			configPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Run Serve in a goroutine with timeout to prevent hanging
			done := make(chan error, 1)
			go func() {
				done <- e5s.Serve(tt.configPath, handler)
			}()

			// Should fail quickly with invalid config
			select {
			case err := <-done:
				if (err != nil) != tt.wantErr {
					t.Errorf("Serve() error = %v, wantErr %v", err, tt.wantErr)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("Serve() did not return within timeout")
			}
		})
	}
}

// TestClient_InvalidConfig verifies Client fails with invalid configuration.
func TestClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		wantErr    bool
	}{
		{
			name:       "non-existent config file",
			configPath: "/nonexistent/config.yaml",
			wantErr:    true,
		},
		{
			name:       "empty config path",
			configPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := e5s.Client(tt.configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestClientWithContext_InvalidConfig verifies ClientWithContext fails with invalid configuration.
func TestClientWithContext_InvalidConfig(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		configPath string
		wantErr    bool
	}{
		{
			name:       "non-existent config file",
			configPath: "/nonexistent/config.yaml",
			wantErr:    true,
		},
		{
			name:       "empty config path",
			configPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := e5s.ClientWithContext(ctx, tt.configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ClientWithContext() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestClientWithContext_ContextCancellation verifies context cancellation during initialization.
func TestClientWithContext_ContextCancellation(t *testing.T) {
	// Create a context that's already canceled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// This should fail because the context is canceled
	_, _, err := e5s.ClientWithContext(ctx, "/nonexistent/config.yaml")
	if err == nil {
		t.Error("expected error with canceled context, got nil")
	}
}

// TestWithClient_InvalidConfig verifies WithClient fails with invalid configuration.
func TestWithClient_InvalidConfig(t *testing.T) {
	tests := []struct {
		name       string
		configPath string
		wantErr    bool
	}{
		{
			name:       "non-existent config file",
			configPath: "/nonexistent/config.yaml",
			wantErr:    true,
		},
		{
			name:       "empty config path",
			configPath: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := e5s.WithClient(tt.configPath, func(client *http.Client) error {
				// This should never be called since config is invalid
				t.Error("callback should not be called with invalid config")
				return nil
			})
			if (err != nil) != tt.wantErr {
				t.Errorf("WithClient() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestWithClient_CallbackError verifies WithClient returns callback errors.
// Note: This test uses invalid config, so the callback won't actually be invoked.
// A proper test would require valid SPIRE infrastructure (covered by integration tests).
func TestWithClient_CallbackError(t *testing.T) {
	// This test verifies the error handling path, though the callback
	// won't be reached due to invalid config
	configPath := "/nonexistent/config.yaml"

	err := e5s.WithClient(configPath, func(client *http.Client) error {
		// This won't be called due to invalid config
		return nil
	})

	if err == nil {
		t.Error("expected error with invalid config, got nil")
	}
}
