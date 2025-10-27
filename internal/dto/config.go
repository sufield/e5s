package dto

// Config is runtime configuration.
// Workload registration happens in SPIRE Server.
type Config struct {
	AgentSpiffeID string `json:"agentSpiffeId" yaml:"agentSpiffeId"`
}
