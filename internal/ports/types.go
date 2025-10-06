package ports

import (
	"github.com/pocket/hexagon/spire/internal/domain"
)

// Identity represents a verified identity
type Identity struct {
	IdentityNamespace *domain.IdentityNamespace // Identity format (URI-formatted identifier)
	Name              string                    // Human-readable name
	IdentityDocument  *domain.IdentityDocument  // X.509 or JWT identity document
}

// ProcessIdentity represents process-level identity attributes for attestation (maps to domain.Workload).
type ProcessIdentity struct {
	PID  int    // Process ID
	UID  int    // User ID
	GID  int    // Group ID
	Path string // Executable path
}

// ToWorkload converts to domain.Workload
func (p ProcessIdentity) ToWorkload() *domain.Workload {
	return domain.NewWorkload(p.PID, p.UID, p.GID, p.Path)
}

// Message represents an authenticated message exchange
type Message struct {
	From    Identity
	To      Identity
	Content string
}

// Config represents application configuration
type Config struct {
	TrustDomain   string
	AgentSpiffeID string
	Workloads     []WorkloadEntry
}

// WorkloadEntry represents a workload entry for registration (e.g., from config or mocks):
// associates UID (for attestation), selector (for matching), and SpiffeID string (the issued identity namespace).
type WorkloadEntry struct {
	SpiffeID string
	Selector string
	UID      int
}
