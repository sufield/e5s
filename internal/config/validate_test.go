package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateServer(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ServerFileConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with client SPIFFE ID",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: ServerSection{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "spiffe://example.org/client",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with trust domain",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: ServerSection{
					ListenAddr:               ":8443",
					AllowedClientTrustDomain: "example.org",
				},
			},
			wantErr: false,
		},
		{
			name: "missing workload socket",
			cfg: ServerFileConfig{
				Server: ServerSection{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "spiffe://example.org/client",
				},
			},
			wantErr: true,
			errMsg:  "workload_socket must be set",
		},
		{
			name: "missing listen address",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: ServerSection{
					AllowedClientSPIFFEID: "spiffe://example.org/client",
				},
			},
			wantErr: true,
			errMsg:  "listen_addr must be set",
		},
		{
			name: "missing both client ID and trust domain",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: ServerSection{
					ListenAddr: ":8443",
				},
			},
			wantErr: true,
			errMsg:  "must set exactly one",
		},
		{
			name: "both client ID and trust domain set",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: ServerSection{
					ListenAddr:               ":8443",
					AllowedClientSPIFFEID:    "spiffe://example.org/client",
					AllowedClientTrustDomain: "example.org",
				},
			},
			wantErr: true,
			errMsg:  "cannot set both",
		},
		{
			name: "invalid SPIFFE ID format",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: ServerSection{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "not-a-valid-spiffe-id",
				},
			},
			wantErr: true,
			errMsg:  "invalid server.allowed_client_spiffe_id",
		},
		{
			name: "invalid trust domain format",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: ServerSection{
					ListenAddr:               ":8443",
					AllowedClientTrustDomain: "invalid domain!",
				},
			},
			wantErr: true,
			errMsg:  "invalid server.allowed_client_trust_domain",
		},
		{
			name: "whitespace trimming for SPIFFE ID",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "  /run/spire/sockets/agent.sock  ",
				},
				Server: ServerSection{
					ListenAddr:            "  :8443  ",
					AllowedClientSPIFFEID: "  spiffe://example.org/client  ",
				},
			},
			wantErr: false,
		},
		{
			name: "whitespace trimming for trust domain",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "  /run/spire/sockets/agent.sock  ",
				},
				Server: ServerSection{
					ListenAddr:               "  :8443  ",
					AllowedClientTrustDomain: "  example.org  ",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spireConfig, authz, err := ValidateServerConfig(&tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateServer() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateServer() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateServer() unexpected error = %v", err)
					return
				}
				// Verify we got a valid SPIRE config
				if spireConfig.InitialFetchTimeout <= 0 {
					t.Errorf("ValidateServer() returned invalid timeout: %v", spireConfig.InitialFetchTimeout)
				}
				// Verify we got a valid authz policy
				hasID := authz.ID.String() != ""
				hasTD := authz.TrustDomain.String() != ""
				if !hasID && !hasTD {
					t.Error("ValidateServer() returned empty authz policy")
				}
				if hasID && hasTD {
					t.Error("ValidateServer() returned authz with both ID and TrustDomain set")
				}
			}
		})
	}
}

func TestValidateClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     ClientFileConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with server SPIFFE ID",
			cfg: ClientFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: ClientSection{
					ExpectedServerSPIFFEID: "spiffe://example.org/server",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with trust domain",
			cfg: ClientFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: ClientSection{
					ExpectedServerTrustDomain: "example.org",
				},
			},
			wantErr: false,
		},
		{
			name: "missing workload socket",
			cfg: ClientFileConfig{
				Client: ClientSection{
					ExpectedServerSPIFFEID: "spiffe://example.org/server",
				},
			},
			wantErr: true,
			errMsg:  "workload_socket must be set",
		},
		{
			name: "missing both server ID and trust domain",
			cfg: ClientFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
			},
			wantErr: true,
			errMsg:  "must set exactly one",
		},
		{
			name: "both server ID and trust domain set",
			cfg: ClientFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: ClientSection{
					ExpectedServerSPIFFEID:    "spiffe://example.org/server",
					ExpectedServerTrustDomain: "example.org",
				},
			},
			wantErr: true,
			errMsg:  "cannot set both",
		},
		{
			name: "invalid SPIFFE ID format",
			cfg: ClientFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: ClientSection{
					ExpectedServerSPIFFEID: "not-a-valid-spiffe-id",
				},
			},
			wantErr: true,
			errMsg:  "invalid client.expected_server_spiffe_id",
		},
		{
			name: "invalid trust domain format",
			cfg: ClientFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: ClientSection{
					ExpectedServerTrustDomain: "invalid domain!",
				},
			},
			wantErr: true,
			errMsg:  "invalid client.expected_server_trust_domain",
		},
		{
			name: "whitespace trimming for SPIFFE ID",
			cfg: ClientFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "  /run/spire/sockets/agent.sock  ",
				},
				Client: ClientSection{
					ExpectedServerSPIFFEID: "  spiffe://example.org/server  ",
				},
			},
			wantErr: false,
		},
		{
			name: "whitespace trimming for trust domain",
			cfg: ClientFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "  /run/spire/sockets/agent.sock  ",
				},
				Client: ClientSection{
					ExpectedServerTrustDomain: "  example.org  ",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spireConfig, authz, err := ValidateClientConfig(&tt.cfg)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ValidateClient() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateClient() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("ValidateClient() unexpected error = %v", err)
					return
				}
				// Verify we got a valid SPIRE config
				if spireConfig.InitialFetchTimeout <= 0 {
					t.Errorf("ValidateClient() returned invalid timeout: %v", spireConfig.InitialFetchTimeout)
				}
				// Verify we got a valid authz policy
				hasID := authz.ID.String() != ""
				hasTD := authz.TrustDomain.String() != ""
				if !hasID && !hasTD {
					t.Error("ValidateClient() returned empty authz policy")
				}
				if hasID && hasTD {
					t.Error("ValidateClient() returned authz with both ID and TrustDomain set")
				}
			}
		})
	}
}

func TestValidateSPIREConfig_Timeout(t *testing.T) {
	tests := []struct {
		name            string
		cfg             ServerFileConfig
		wantErr         bool
		errMsg          string
		expectedTimeout time.Duration
	}{
		{
			name: "default timeout when not specified",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
					// InitialFetchTimeout not set
				},
			},
			wantErr:         false,
			expectedTimeout: DefaultInitialFetchTimeout,
		},
		{
			name: "valid timeout - seconds",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket:      "/run/spire/sockets/agent.sock",
					InitialFetchTimeout: "10s",
				},
			},
			wantErr:         false,
			expectedTimeout: 10 * time.Second,
		},
		{
			name: "valid timeout - minutes",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket:      "/run/spire/sockets/agent.sock",
					InitialFetchTimeout: "2m",
				},
			},
			wantErr:         false,
			expectedTimeout: 2 * time.Minute,
		},
		{
			name: "invalid timeout format",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket:      "/run/spire/sockets/agent.sock",
					InitialFetchTimeout: "not-a-duration",
				},
			},
			wantErr: true,
			errMsg:  "invalid spire.initial_fetch_timeout",
		},
		{
			name: "negative timeout",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket:      "/run/spire/sockets/agent.sock",
					InitialFetchTimeout: "-5s",
				},
			},
			wantErr: true,
			errMsg:  "must be positive",
		},
		{
			name: "zero timeout",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket:      "/run/spire/sockets/agent.sock",
					InitialFetchTimeout: "0s",
				},
			},
			wantErr: true,
			errMsg:  "must be positive",
		},
		{
			name: "whitespace trimming for timeout",
			cfg: ServerFileConfig{
				SPIRE: SPIRESection{
					WorkloadSocket:      "/run/spire/sockets/agent.sock",
					InitialFetchTimeout: "  15s  ",
				},
			},
			wantErr:         false,
			expectedTimeout: 15 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spireConfig, err := validateSPIRESection(tt.cfg.SPIRE)
			if tt.wantErr {
				if err == nil {
					t.Errorf("validateSPIREConfig() expected error containing %q, got nil", tt.errMsg)
					return
				}
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("validateSPIREConfig() error = %q, want error containing %q", err.Error(), tt.errMsg)
				}
			} else {
				if err != nil {
					t.Errorf("validateSPIREConfig() unexpected error = %v", err)
					return
				}
				if spireConfig.InitialFetchTimeout != tt.expectedTimeout {
					t.Errorf("validateSPIREConfig() timeout = %v, want %v", spireConfig.InitialFetchTimeout, tt.expectedTimeout)
				}
			}
		})
	}
}
