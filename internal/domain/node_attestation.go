//go:build !production

package domain

// NOTE: This file (node_attestation.go) is ONLY used by the in-memory implementation.
// In production deployments using real SPIRE, node attestation is handled by SPIRE Server.
// This file is excluded from production builds via build tag.

// NodeAttestationResult represents the result of node attestation
type NodeAttestationResult struct {
	node      *Node
	selectors *SelectorSet
	attested  bool
}

// NewNodeAttestationResult creates a new node attestation result
func NewNodeAttestationResult(node *Node, selectors *SelectorSet, attested bool) *NodeAttestationResult {
	return &NodeAttestationResult{
		node:      node,
		selectors: selectors,
		attested:  attested,
	}
}

// Node returns the attested node
func (r *NodeAttestationResult) Node() *Node {
	return r.node
}

// Selectors returns the node selectors
func (r *NodeAttestationResult) Selectors() *SelectorSet {
	return r.selectors
}

// Attested returns whether attestation succeeded
func (r *NodeAttestationResult) Attested() bool {
	return r.attested
}
