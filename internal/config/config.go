package config

// SPIRESection contains SPIRE Workload API configuration.
// This is shared between server and client processes.
type SPIRESection struct {
	// WorkloadSocket is the path to the SPIRE Agent's Workload API socket.
	// Example: "unix:///tmp/spire-agent/public/api.sock"
	WorkloadSocket string `yaml:"workload_socket"`

	// InitialFetchTimeout is how long to wait for the first SVID/Bundle from
	// the Workload API before giving up and failing startup.
	// Use Go duration format: "5s", "30s", "1m", etc.
	// If not set, defaults to 30 seconds.
	InitialFetchTimeout string `yaml:"initial_fetch_timeout"`
}

// ServerSection contains server-specific configuration.
type ServerSection struct {
	ListenAddr               string `yaml:"listen_addr"`
	AllowedClientSPIFFEID    string `yaml:"allowed_client_spiffe_id"`
	AllowedClientTrustDomain string `yaml:"allowed_client_trust_domain"`
}

// ClientSection contains client-specific configuration.
type ClientSection struct {
	ExpectedServerSPIFFEID    string `yaml:"expected_server_spiffe_id"`
	ExpectedServerTrustDomain string `yaml:"expected_server_trust_domain"`
}

// ServerFileConfig represents an e5s server configuration file.
// This config is used by processes that listen for mTLS connections (servers).
//
// The config format is versioned to support future evolution without breaking changes.
type ServerFileConfig struct {
	// Version is the config file format version (optional, currently always 1)
	// Future versions may add/change fields while maintaining backward compatibility.
	Version int `yaml:"version,omitempty"`

	SPIRE  SPIRESection  `yaml:"spire"`
	Server ServerSection `yaml:"server"`
}

// ClientFileConfig represents an e5s client configuration file.
// This config is used by processes that make mTLS connections (clients).
//
// The config format is versioned to support future evolution without breaking changes.
type ClientFileConfig struct {
	// Version is the config file format version (optional, currently always 1)
	// Future versions may add/change fields while maintaining backward compatibility.
	Version int `yaml:"version,omitempty"`

	SPIRE  SPIRESection  `yaml:"spire"`
	Client ClientSection `yaml:"client"`
}

// FileConfig represents the legacy combined configuration file structure.
// DEPRECATED: Use ServerFileConfig or ClientFileConfig instead.
// This type exists only for backward compatibility and will be removed in a future version.
type FileConfig struct {
	Version int           `yaml:"version,omitempty"`
	SPIRE   SPIRESection  `yaml:"spire"`
	Server  ServerSection `yaml:"server"`
	Client  ClientSection `yaml:"client"`
}
