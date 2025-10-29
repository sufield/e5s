package config

import (
	"strings"
	"testing"
)

func TestValidateServer(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FileConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with client SPIFFE ID",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "spiffe://example.org/client",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with trust domain",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					ListenAddr:               ":8443",
					AllowedClientTrustDomain: "example.org",
				},
			},
			wantErr: false,
		},
		{
			name: "missing workload socket",
			cfg: FileConfig{
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "spiffe://example.org/client",
				},
			},
			wantErr: true,
			errMsg:  "workload_socket must be set",
		},
		{
			name: "missing listen address",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					AllowedClientSPIFFEID: "spiffe://example.org/client",
				},
			},
			wantErr: true,
			errMsg:  "listen_addr must be set",
		},
		{
			name: "missing both client ID and trust domain",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					ListenAddr: ":8443",
				},
			},
			wantErr: true,
			errMsg:  "must set exactly one",
		},
		{
			name: "both client ID and trust domain set",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
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
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "not-a-valid-spiffe-id",
				},
			},
			wantErr: true,
			errMsg:  "invalid server.allowed_client_spiffe_id",
		},
		{
			name: "invalid trust domain format",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					ListenAddr:               ":8443",
					AllowedClientTrustDomain: "invalid domain!",
				},
			},
			wantErr: true,
			errMsg:  "invalid server.allowed_client_trust_domain",
		},
		{
			name: "whitespace trimming for SPIFFE ID",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "  /run/spire/sockets/agent.sock  ",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					ListenAddr:            "  :8443  ",
					AllowedClientSPIFFEID: "  spiffe://example.org/client  ",
				},
			},
			wantErr: false,
		},
		{
			name: "whitespace trimming for trust domain",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "  /run/spire/sockets/agent.sock  ",
				},
				Server: struct {
					ListenAddr               string `yaml:"listen_addr"`
					AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
					AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
				}{
					ListenAddr:               "  :8443  ",
					AllowedClientTrustDomain: "  example.org  ",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authz, err := ValidateServer(tt.cfg)
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
		cfg     FileConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with server SPIFFE ID",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: struct {
					ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
					ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
				}{
					ExpectedServerSPIFFEID: "spiffe://example.org/server",
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with trust domain",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: struct {
					ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
					ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
				}{
					ExpectedServerTrustDomain: "example.org",
				},
			},
			wantErr: false,
		},
		{
			name: "missing workload socket",
			cfg: FileConfig{
				Client: struct {
					ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
					ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
				}{
					ExpectedServerSPIFFEID: "spiffe://example.org/server",
				},
			},
			wantErr: true,
			errMsg:  "workload_socket must be set",
		},
		{
			name: "missing both server ID and trust domain",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
			},
			wantErr: true,
			errMsg:  "must set exactly one",
		},
		{
			name: "both server ID and trust domain set",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: struct {
					ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
					ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
				}{
					ExpectedServerSPIFFEID:    "spiffe://example.org/server",
					ExpectedServerTrustDomain: "example.org",
				},
			},
			wantErr: true,
			errMsg:  "cannot set both",
		},
		{
			name: "invalid SPIFFE ID format",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: struct {
					ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
					ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
				}{
					ExpectedServerSPIFFEID: "not-a-valid-spiffe-id",
				},
			},
			wantErr: true,
			errMsg:  "invalid client.expected_server_spiffe_id",
		},
		{
			name: "invalid trust domain format",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "/run/spire/sockets/agent.sock",
				},
				Client: struct {
					ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
					ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
				}{
					ExpectedServerTrustDomain: "invalid domain!",
				},
			},
			wantErr: true,
			errMsg:  "invalid client.expected_server_trust_domain",
		},
		{
			name: "whitespace trimming for SPIFFE ID",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "  /run/spire/sockets/agent.sock  ",
				},
				Client: struct {
					ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
					ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
				}{
					ExpectedServerSPIFFEID: "  spiffe://example.org/server  ",
				},
			},
			wantErr: false,
		},
		{
			name: "whitespace trimming for trust domain",
			cfg: FileConfig{
				SPIRE: struct {
					WorkloadSocket string `yaml:"workload_socket"`
				}{
					WorkloadSocket: "  /run/spire/sockets/agent.sock  ",
				},
				Client: struct {
					ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
					ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
				}{
					ExpectedServerTrustDomain: "  example.org  ",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authz, err := ValidateClient(tt.cfg)
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
