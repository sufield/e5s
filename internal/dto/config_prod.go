//go:build !dev

package dto

// Config is runtime configuration (production).
// Production only needs the agent SPIFFE ID; workload registration happens in SPIRE Server.
type Config struct {
	AgentSpiffeID string `json:"agentSpiffeId" yaml:"agentSpiffeId"`
}
