package config

import (
	"strings"
	"testing"
)

func TestValidateServer(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FileConfig
		wantErr string
	}{
		{
			name: "valid with client SPIFFE ID",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Server: ServerConfig{
					ListenAddr:             ":8443",
					AllowedClientSPIFFEID:  "spiffe://example.org/client",
					AllowedClientTrustDomain: "",
				},
			},
			wantErr: "",
		},
		{
			name: "valid with trust domain",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Server: ServerConfig{
					ListenAddr:               ":8443",
					AllowedClientSPIFFEID:    "",
					AllowedClientTrustDomain: "example.org",
				},
			},
			wantErr: "",
		},
		{
			name: "missing workload socket",
			cfg: FileConfig{
				Server: ServerConfig{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "spiffe://example.org/client",
				},
			},
			wantErr: "spire.workload_socket must be set",
		},
		{
			name: "missing listen address",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Server: ServerConfig{
					AllowedClientSPIFFEID: "spiffe://example.org/client",
				},
			},
			wantErr: "server.listen_addr must be set",
		},
		{
			name: "neither policy set",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Server: ServerConfig{
					ListenAddr: ":8443",
				},
			},
			wantErr: "must set exactly one of server.allowed_client_spiffe_id or server.allowed_client_trust_domain",
		},
		{
			name: "both policies set",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Server: ServerConfig{
					ListenAddr:               ":8443",
					AllowedClientSPIFFEID:    "spiffe://example.org/client",
					AllowedClientTrustDomain: "example.org",
				},
			},
			wantErr: "cannot set both",
		},
		{
			name: "invalid SPIFFE ID format",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Server: ServerConfig{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "not-a-valid-spiffe-id",
				},
			},
			wantErr: "invalid server.allowed_client_spiffe_id",
		},
		{
			name: "invalid trust domain format",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Server: ServerConfig{
					ListenAddr:               ":8443",
					AllowedClientTrustDomain: "invalid/trust/domain",
				},
			},
			wantErr: "invalid server.allowed_client_trust_domain",
		},
		{
			name: "SPIFFE ID with path is valid",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Server: ServerConfig{
					ListenAddr:            ":8443",
					AllowedClientSPIFFEID: "spiffe://example.org/ns/default/sa/client",
				},
			},
			wantErr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServer(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateServer() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidateServer() error = nil, want error containing %q", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ValidateServer() error = %v, want error containing %q", err, tt.wantErr)
				}
			}
		})
	}
}

func TestValidateClient(t *testing.T) {
	tests := []struct {
		name    string
		cfg     FileConfig
		wantErr string
	}{
		{
			name: "valid with server SPIFFE ID",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Client: ClientConfig{
					ExpectedServerSPIFFEID:    "spiffe://example.org/server",
					ExpectedServerTrustDomain: "",
				},
			},
			wantErr: "",
		},
		{
			name: "valid with trust domain",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Client: ClientConfig{
					ExpectedServerSPIFFEID:    "",
					ExpectedServerTrustDomain: "example.org",
				},
			},
			wantErr: "",
		},
		{
			name: "missing workload socket",
			cfg: FileConfig{
				Client: ClientConfig{
					ExpectedServerSPIFFEID: "spiffe://example.org/server",
				},
			},
			wantErr: "spire.workload_socket must be set",
		},
		{
			name: "neither policy set",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Client: ClientConfig{},
			},
			wantErr: "must set exactly one of client.expected_server_spiffe_id or client.expected_server_trust_domain",
		},
		{
			name: "both policies set",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Client: ClientConfig{
					ExpectedServerSPIFFEID:    "spiffe://example.org/server",
					ExpectedServerTrustDomain: "example.org",
				},
			},
			wantErr: "cannot set both",
		},
		{
			name: "invalid SPIFFE ID format",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Client: ClientConfig{
					ExpectedServerSPIFFEID: "http://example.org/server",
				},
			},
			wantErr: "invalid client.expected_server_spiffe_id",
		},
		{
			name: "invalid trust domain format",
			cfg: FileConfig{
				SPIRE: SPIREConfig{
					WorkloadSocket: "unix:///tmp/agent.sock",
				},
				Client: ClientConfig{
					ExpectedServerTrustDomain: "INVALID_TRUST_DOMAIN",
				},
			},
			wantErr: "invalid client.expected_server_trust_domain",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateClient(tt.cfg)
			if tt.wantErr == "" {
				if err != nil {
					t.Errorf("ValidateClient() error = %v, want nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidateClient() error = nil, want error containing %q", tt.wantErr)
				} else if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("ValidateClient() error = %v, want error containing %q", err, tt.wantErr)
				}
			}
		})
	}
}
