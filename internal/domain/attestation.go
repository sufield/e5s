package domain

// NOTE: This file (attestation.go) is primarily used by the in-memory implementation.
// In production deployments using real SPIRE, attestation is handled by SPIRE Agent/Server.
// However, this file must remain included in production builds because:
// 1. AttestationService.MatchWorkloadToMapper() provides selector matching domain logic
// 2. WorkloadAttestationResult uses SelectorSet (required type)
// 3. These types complete the domain model vocabulary (see doc.go for full domain overview)
// Production code delegates all attestation to external SPIRE infrastructure.
//
// NOTE: NodeAttestationResult has been moved to node_attestation.go with !production build tag.

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
