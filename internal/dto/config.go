//go:build dev

package dto

// Config is runtime configuration (dev-only).
type Config struct {
	TrustDomain   string          `json:"trustDomain" yaml:"trustDomain"`
	AgentSpiffeID string          `json:"agentSpiffeId" yaml:"agentSpiffeId"`
	Workloads     []WorkloadEntry `json:"workloads,omitempty" yaml:"workloads,omitempty"`
}
