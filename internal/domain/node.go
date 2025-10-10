//go:build !production

package domain

// NOTE: This file (node.go) is ONLY used by the in-memory implementation.
// In production deployments using real SPIRE, node attestation is handled by SPIRE Server.
// This file is excluded from production builds via build tag.

// Node represents the host machine or environment where the agent and workloads run
// Its identity is verified via node attestation
type Node struct {
	identityCredential *IdentityCredential
	selectors          *SelectorSet
	attested           bool
}

// NewNode creates a new node
func NewNode(identityCredential *IdentityCredential) *Node {
	return &Node{
		identityCredential: identityCredential,
		selectors:          NewSelectorSet(),
		attested:           false,
	}
}

// IdentityCredential returns the node's identity credential
func (n *Node) IdentityCredential() *IdentityCredential {
	return n.identityCredential
}

// Selectors returns the node's selectors
func (n *Node) Selectors() *SelectorSet {
	return n.selectors
}

// SetSelectors sets the node's selectors (from attestation)
func (n *Node) SetSelectors(selectors *SelectorSet) {
	n.selectors = selectors
}

// MarkAttested marks the node as attested
func (n *Node) MarkAttested() {
	n.attested = true
}

// IsAttested checks if the node has been attested
func (n *Node) IsAttested() bool {
	return n.attested
}
