package domain

// Node represents the host machine or environment where the agent and workloads run
// Its identity is verified via node attestation
type Node struct {
	identityNamespace *IdentityNamespace
	selectors         *SelectorSet
	attested          bool
}

// NewNode creates a new node
func NewNode(identityNamespace *IdentityNamespace) *Node {
	return &Node{
		identityNamespace: identityNamespace,
		selectors:         NewSelectorSet(),
		attested:          false,
	}
}

// IdentityNamespace returns the node's identity namespace
func (n *Node) IdentityNamespace() *IdentityNamespace {
	return n.identityNamespace
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
