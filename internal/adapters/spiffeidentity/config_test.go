package spiffeidentity

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with trust domain",
			config: Config{
				WorkloadAPISocket:   "unix:///tmp/spire-agent.sock",
				ExpectedTrustDomain: "dev.local",
			},
			wantErr: false,
		},
		{
			name: "valid config without trust domain",
			config: Config{
				WorkloadAPISocket: "unix:///tmp/spire-agent.sock",
			},
			wantErr: false,
		},
		{
			name: "valid config with empty trust domain",
			config: Config{
				WorkloadAPISocket:   "unix:///tmp/spire-agent.sock",
				ExpectedTrustDomain: "",
			},
			wantErr: false,
		},
		{
			name: "empty socket path",
			config: Config{
				WorkloadAPISocket:   "",
				ExpectedTrustDomain: "dev.local",
			},
			wantErr: true,
			errMsg:  "WorkloadAPISocket is required",
		},
		{
			name: "socket path without unix:// prefix",
			config: Config{
				WorkloadAPISocket:   "/tmp/spire-agent.sock",
				ExpectedTrustDomain: "dev.local",
			},
			wantErr: true,
			errMsg:  "must start with 'unix://'",
		},
		{
			name: "trust domain with scheme",
			config: Config{
				WorkloadAPISocket:   "unix:///tmp/spire-agent.sock",
				ExpectedTrustDomain: "spiffe://dev.local",
			},
			wantErr: true,
			errMsg:  "must not include scheme",
		},
		{
			name: "trust domain with path separator",
			config: Config{
				WorkloadAPISocket:   "unix:///tmp/spire-agent.sock",
				ExpectedTrustDomain: "dev.local/path",
			},
			wantErr: true,
			errMsg:  "must not include path separators",
		},
		{
			name: "trust domain with invalid characters",
			config: Config{
				WorkloadAPISocket:   "unix:///tmp/spire-agent.sock",
				ExpectedTrustDomain: "dev@local",
			},
			wantErr: true,
			errMsg:  "invalid character",
		},
		{
			name: "trust domain exceeds max length",
			config: Config{
				WorkloadAPISocket:   "unix:///tmp/spire-agent.sock",
				ExpectedTrustDomain: strings.Repeat("a", 254),
			},
			wantErr: true,
			errMsg:  "exceeds maximum length 253",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.ErrorContains(t, err, tt.errMsg)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestLoadFromEnv(t *testing.T) {
	tests := []struct {
		name        string
		setupEnv    func(*testing.T)
		want        Config
		wantErr     bool
		errMsg      string
	}{
		{
			name: "valid environment with trust domain",
			setupEnv: func(t *testing.T) {
				t.Setenv("SPIFFE_WORKLOAD_API_SOCKET", "unix:///tmp/spire-agent.sock")
				t.Setenv("SPIFFE_TRUST_DOMAIN", "dev.local")
			},
			want: Config{
				WorkloadAPISocket:   "unix:///tmp/spire-agent.sock",
				ExpectedTrustDomain: "dev.local",
			},
			wantErr: false,
		},
		{
			name: "valid environment without trust domain",
			setupEnv: func(t *testing.T) {
				t.Setenv("SPIFFE_WORKLOAD_API_SOCKET", "unix:///tmp/spire-agent.sock")
			},
			want: Config{
				WorkloadAPISocket: "unix:///tmp/spire-agent.sock",
			},
			wantErr: false,
		},
		{
			name: "env with extra whitespace",
			setupEnv: func(t *testing.T) {
				t.Setenv("SPIFFE_WORKLOAD_API_SOCKET", " unix:///tmp/spire-agent.sock ")
				t.Setenv("SPIFFE_TRUST_DOMAIN", " dev.local ")
			},
			want: Config{
				WorkloadAPISocket:   "unix:///tmp/spire-agent.sock",
				ExpectedTrustDomain: "dev.local",
			},
			wantErr: false,
		},
		{
			name: "missing socket environment variable",
			setupEnv: func(t *testing.T) {
				// Don't set any environment variables
			},
			wantErr: true,
			errMsg:  "SPIFFE_WORKLOAD_API_SOCKET environment variable is required",
		},
		{
			name: "invalid socket path format",
			setupEnv: func(t *testing.T) {
				t.Setenv("SPIFFE_WORKLOAD_API_SOCKET", "/tmp/spire-agent.sock")
			},
			wantErr: true,
			errMsg:  "must start with 'unix://'",
		},
		{
			name: "trust domain with scheme",
			setupEnv: func(t *testing.T) {
				t.Setenv("SPIFFE_WORKLOAD_API_SOCKET", "unix:///tmp/spire-agent.sock")
				t.Setenv("SPIFFE_TRUST_DOMAIN", "spiffe://dev.local")
			},
			wantErr: true,
			errMsg:  "must not include scheme",
		},
		{
			name: "trust domain with path separator",
			setupEnv: func(t *testing.T) {
				t.Setenv("SPIFFE_WORKLOAD_API_SOCKET", "unix:///tmp/spire-agent.sock")
				t.Setenv("SPIFFE_TRUST_DOMAIN", "dev.local/path")
			},
			wantErr: true,
			errMsg:  "must not include path separators",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() with t.Setenv()
			// Setup environment (t.Setenv handles cleanup automatically)
			tt.setupEnv(t)

			// Load config
			got, err := LoadFromEnv()

			// Check error
			if tt.wantErr {
				assert.Error(t, err)
				if tt.errMsg != "" {
					assert.ErrorContains(t, err, tt.errMsg)
				}
				return
			}

			// Check config values
			assert.NoError(t, err)
			assert.Equal(t, tt.want.WorkloadAPISocket, got.WorkloadAPISocket)
			assert.Equal(t, tt.want.ExpectedTrustDomain, got.ExpectedTrustDomain)
		})
	}
}

func TestLoadFromEnv_ProductionExamples(t *testing.T) {
	tests := []struct {
		name       string
		socket     string
		trustDomain string
		description string
	}{
		{
			name:        "dev unix attestor",
			socket:      "unix:///tmp/spire-agent/public/api.sock",
			trustDomain: "dev.local",
			description: "Development environment with unix workload attestor",
		},
		{
			name:        "prod kubernetes",
			socket:      "unix:///spiffe-workload-api/spire-agent.sock",
			trustDomain: "prod.example.com",
			description: "Production Kubernetes with k8s workload attestor",
		},
		{
			name:        "prod aws",
			socket:      "unix:///run/spire/sockets/agent.sock",
			trustDomain: "prod.example.com",
			description: "Production AWS with aws workload attestor",
		},
		{
			name:        "prod azure",
			socket:      "unix:///var/run/spire/sockets/agent.sock",
			trustDomain: "prod.example.com",
			description: "Production Azure with azure workload attestor",
		},
		{
			name:        "prod gcp",
			socket:      "unix:///run/spire/agent.sock",
			trustDomain: "prod.example.com",
			description: "Production GCP with gcp workload attestor",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Note: Cannot use t.Parallel() with t.Setenv()
			// Setup (t.Setenv handles cleanup automatically)
			t.Setenv("SPIFFE_WORKLOAD_API_SOCKET", tt.socket)
			t.Setenv("SPIFFE_TRUST_DOMAIN", tt.trustDomain)

			// Load config
			cfg, err := LoadFromEnv()
			assert.NoError(t, err, "LoadFromEnv() failed for %s", tt.description)

			// Validate config
			err = cfg.Validate()
			assert.NoError(t, err, "Config validation failed for %s", tt.description)

			// Check values
			assert.Equal(t, tt.socket, cfg.WorkloadAPISocket)
			assert.Equal(t, tt.trustDomain, cfg.ExpectedTrustDomain)
		})
	}
}

// BenchmarkLoadFromEnv benchmarks the LoadFromEnv function
func BenchmarkLoadFromEnv(b *testing.B) {
	// Setup environment once
	os.Setenv("SPIFFE_WORKLOAD_API_SOCKET", "unix:///tmp/spire-agent.sock")
	os.Setenv("SPIFFE_TRUST_DOMAIN", "dev.local")
	defer func() {
		os.Unsetenv("SPIFFE_WORKLOAD_API_SOCKET")
		os.Unsetenv("SPIFFE_TRUST_DOMAIN")
	}()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = LoadFromEnv()
	}
}
