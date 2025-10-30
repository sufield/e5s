package config

// FileConfig represents the complete e5s configuration file structure.
// This single config file is used by both server and client processes.
type FileConfig struct {
	SPIRE struct {
		// WorkloadSocket is the path to the SPIRE Agent's Workload API socket.
		// Example: "unix:///tmp/spire-agent/public/api.sock"
		WorkloadSocket string `yaml:"workload_socket"`

		// InitialFetchTimeout is how long to wait for the first SVID/Bundle from
		// the Workload API before giving up and failing startup.
		// Use Go duration format: "5s", "30s", "1m", etc.
		// If not set, defaults to 30 seconds.
		InitialFetchTimeout string `yaml:"initial_fetch_timeout"`
	} `yaml:"spire"`

	Server struct {
		ListenAddr               string `yaml:"listen_addr"`
		AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
		AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
	} `yaml:"server"`

	Client struct {
		ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
		ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
	} `yaml:"client"`
}
