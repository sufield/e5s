package debug

// Snapshot is what we expose over /_debug/identity.
//
// ABSOLUTE RULE: This struct (and any nested structs like CertView/AuthDecision)
// MUST NEVER contain:
//   - private keys
//   - raw cert material / full PEM / JWTs / bearer tokens
//   - socket paths or network endpoints that are not already public-facing
//
// This file is intentionally built in ALL builds (no //go:build tag) so other
// packages can reference these types. The /_debug/identity endpoint that returns
// this data only exists in debug builds, but treating this struct as "safe for
// prod" is a design goal. Adding sensitive material here is a security bug.
type Snapshot struct {
	Mode            string         `json:"mode"`            // "debug", "staging", or "production"
	TrustDomain     string         `json:"trustDomain"`     // e.g., "spiffe://example.org"
	Adapter         string         `json:"adapter"`         // "inmemory" or "spire"
	Certs           []CertView     `json:"certs"`           // Current certificates
	RecentDecisions []AuthDecision `json:"recentDecisions"` // Recent auth decisions
}

// CertView provides a safe view of certificate information.
// Excludes private keys and raw certificate data.
type CertView struct {
	SpiffeID         string `json:"spiffeID"`         // e.g., "spiffe://example.org/server"
	ExpiresInSeconds int64  `json:"expiresInSeconds"` // Time until expiration (negative if expired)
	RotationPending  bool   `json:"rotationPending"`  // True if rotation is scheduled/in progress
}

// AuthDecision represents a single authentication decision.
// Used for debugging authorization logic.
type AuthDecision struct {
	CallerSPIFFEID string `json:"callerSPIFFEID"` // Who tried to authenticate
	Resource       string `json:"resource"`       // What resource was accessed
	Decision       string `json:"decision"`       // "ALLOW" or "DENY"
	Reason         string `json:"reason"`         // Human-readable reason
}
