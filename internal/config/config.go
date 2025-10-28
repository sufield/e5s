package config

// FileConfig represents the complete e5s configuration file structure.
// This single config file is used by both server and client processes.
type FileConfig struct {
	SPIRE struct {
		WorkloadSocket string `yaml:"workload_socket"`
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
