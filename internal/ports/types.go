package ports

import "github.com/pocket/hexagon/spire/internal/domain"

// Identity is a simple transport object crossing the hex boundary.
// No behavior here. Use app-layer helpers for derivations/validation.
type Identity struct {
	IdentityCredential *domain.IdentityCredential `json:"identityCredential" yaml:"identityCredential"`
	Name               string                     `json:"name,omitempty" yaml:"name,omitempty"` // optional convenience
	IdentityDocument   *domain.IdentityDocument   `json:"identityDocument" yaml:"identityDocument"`
}

// ProcessIdentity describes process attributes used for attestation.
// No methods: conversion/validation belong in app/domain.
type ProcessIdentity struct {
	PID  int    `json:"pid" yaml:"pid"`
	UID  int    `json:"uid" yaml:"uid"`
	GID  int    `json:"gid" yaml:"gid"`
	Path string `json:"path" yaml:"path"`
}

// Config is runtime configuration (prod uses subset; dev uses all fields).
type Config struct {
	TrustDomain   string          `json:"trustDomain" yaml:"trustDomain"`     // e.g., "example.org"
	AgentSpiffeID string          `json:"agentSpiffeId" yaml:"agentSpiffeId"` // string form of SPIFFE ID
	Workloads     []WorkloadEntry `json:"workloads,omitempty" yaml:"workloads,omitempty"`
}

// WorkloadEntry is used to seed an in-memory registry (dev) or describe workload configs.
type WorkloadEntry struct {
	SpiffeID string `json:"spiffeId" yaml:"spiffeId"` // "spiffe://example.org/..."
	Selector string `json:"selector" yaml:"selector"` // "type:key:value" (e.g., "unix:uid:1000")
	UID      int    `json:"uid" yaml:"uid"`           // for simple dev attestors
}
