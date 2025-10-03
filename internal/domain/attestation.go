package domain

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

// WorkloadAttestationResult represents the result of workload attestation
type WorkloadAttestationResult struct {
	workload  *Workload
	selectors *SelectorSet
	attested  bool
}

// NewWorkloadAttestationResult creates a new workload attestation result
func NewWorkloadAttestationResult(workload *Workload, selectors *SelectorSet, attested bool) *WorkloadAttestationResult {
	return &WorkloadAttestationResult{
		workload:  workload,
		selectors: selectors,
		attested:  attested,
	}
}

// Workload returns the attested workload
func (r *WorkloadAttestationResult) Workload() *Workload {
	return r.workload
}

// Selectors returns the workload selectors
func (r *WorkloadAttestationResult) Selectors() *SelectorSet {
	return r.selectors
}

// Attested returns whether attestation succeeded
func (r *WorkloadAttestationResult) Attested() bool {
	return r.attested
}

// AttestationService provides domain logic for attestation processes
type AttestationService struct{}

// NewAttestationService creates a new attestation service
func NewAttestationService() *AttestationService {
	return &AttestationService{}
}

// MatchWorkloadToMapper finds an identity mapper matching the workload's selectors
// Returns ErrNoMatchingMapper if no mapper matches the provided selectors
// Returns ErrInvalidSelectors if selectors are nil or empty
func (s *AttestationService) MatchWorkloadToMapper(
	selectors *SelectorSet,
	mappers []*IdentityMapper,
) (*IdentityMapper, error) {
	if selectors == nil || len(selectors.All()) == 0 {
		return nil, ErrInvalidSelectors
	}

	for _, mapper := range mappers {
		if mapper.MatchesSelectors(selectors) {
			return mapper, nil
		}
	}

	return nil, ErrNoMatchingMapper
}
