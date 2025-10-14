//go:build dev

package domain

import "fmt"

// NOTE: This file is only included in development builds (via //go:build dev tag).
// In production deployments using real SPIRE, attestation is handled by SPIRE Agent/Server.
// Production builds exclude this file entirely, as workloads only fetch identities via Workload API.

// WorkloadAttestationResult represents the result of workload attestation.
//
// Immutability Contract:
//   - Workload and SelectorSet are treated as immutable after creation
//   - Returned references point to the original objects (no defensive copying)
//   - Callers MUST NOT modify returned Workload or SelectorSet instances
//   - If mutation is needed, callers should create new instances
//
// This immutability contract avoids allocation overhead while maintaining safety
// when used correctly in the domain layer.
type WorkloadAttestationResult struct {
	workload  *Workload
	selectors *SelectorSet
	attested  bool
}

// NewWorkloadAttestationResult creates a new workload attestation result.
//
// Parameters:
//   - workload: The attested workload (may be nil if attested is false)
//   - selectors: The workload selectors (may be nil if attested is false)
//   - attested: Whether attestation succeeded
//
// The result captures snapshots of the provided workload and selectors.
// These are treated as immutable per the domain's immutability contract.
func NewWorkloadAttestationResult(workload *Workload, selectors *SelectorSet, attested bool) *WorkloadAttestationResult {
	return &WorkloadAttestationResult{
		workload:  workload,
		selectors: selectors,
		attested:  attested,
	}
}

// Workload returns the attested workload.
//
// Returns the internal workload reference per the immutability contract.
// Callers MUST NOT modify the returned workload.
func (r *WorkloadAttestationResult) Workload() *Workload {
	return r.workload
}

// Selectors returns the workload selectors.
//
// Returns the internal SelectorSet reference per the immutability contract.
// Callers MUST NOT modify the returned selector set.
func (r *WorkloadAttestationResult) Selectors() *SelectorSet {
	return r.selectors
}

// Attested returns whether attestation succeeded
func (r *WorkloadAttestationResult) Attested() bool {
	return r.attested
}

// AttestationService provides domain logic for attestation processes.
//
// This service is stateless and safe for concurrent use.
type AttestationService struct{}

// NewAttestationService creates a new attestation service
func NewAttestationService() *AttestationService {
	return &AttestationService{}
}

// MatchWorkloadToMapper finds the most specific identity mapper matching the workload's selectors.
//
// Matching Policy (deterministic, order-independent):
//  1. Specificity: Select mapper with highest number of required selectors (most restrictive)
//  2. Tie-breaking: If multiple mappers have same specificity, pick lexicographically smallest
//     IdentityCredential string (stable, repeatable selection)
//  3. Nil-safe: Skips nil mappers in the input list (defensive)
//
// This policy ensures:
//   - Deterministic selection: same inputs always produce same result
//   - Order-independent: mapper list order doesn't affect selection
//   - Specificity-first: more restrictive mappers take precedence over broad ones
//   - No shadow bugs: specific mappers can't be shadowed by earlier broad matches
//
// Example:
//
//	Mapper A: requires selectors [type:app]                    (1 selector)
//	Mapper B: requires selectors [type:app, env:prod]          (2 selectors)
//	Mapper C: requires selectors [type:app, env:prod]          (2 selectors, but ID > B's ID)
//
//	Workload has selectors: [type:app, env:prod, region:us]
//
//	All three match, but B wins (highest specificity=2, smaller ID than C).
//
// Error Handling:
//   - Returns ErrInvalidSelectors (wrapped with %w) if selectors nil/empty
//   - Returns ErrNoMatchingMapper (wrapped with %w and context) if no mapper matches
//   - Both sentinels allow errors.Is checking by callers
//
// Parameters:
//   - selectors: The workload's selector set (must be non-nil with at least one selector)
//   - mappers: Candidate identity mappers (may contain nil entries, which are skipped)
//
// Returns:
//   - The most specific matching mapper (never nil on success)
//   - Error if selectors invalid or no mapper matches
//
// Concurrency: Safe for concurrent use (stateless, pure function).
func (s *AttestationService) MatchWorkloadToMapper(
	selectors *SelectorSet,
	mappers []*IdentityMapper,
) (*IdentityMapper, error) {
	// Validate selectors
	if selectors == nil || len(selectors.All()) == 0 {
		return nil, fmt.Errorf("%w: selectors are nil or empty", ErrInvalidSelectors)
	}

	// Validate mappers list
	if len(mappers) == 0 {
		return nil, fmt.Errorf("%w: no mappers provided", ErrNoMatchingMapper)
	}

	// Find best match using specificity + tie-breaking
	var best *IdentityMapper
	bestScore := -1

	for _, mapper := range mappers {
		// Skip nil mappers (defensive)
		if mapper == nil {
			continue
		}

		// Check if mapper matches the selectors
		if !mapper.MatchesSelectors(selectors) {
			continue
		}

		// Calculate specificity score (number of required selectors)
		score := mapperSpecificity(mapper)

		// Update best match based on specificity and tie-breaking
		switch {
		case score > bestScore:
			// Higher specificity wins
			best = mapper
			bestScore = score

		case score == bestScore && best != nil:
			// Tie-breaker: lexicographically smaller IdentityCredential string
			// This ensures deterministic, repeatable selection
			if mapper.IdentityCredential().String() < best.IdentityCredential().String() {
				best = mapper
			}
		}
	}

	// Check if any mapper matched
	if best == nil {
		// Include selector details in error for better debugging
		return nil, fmt.Errorf("%w: no mapper matched selectors %v", ErrNoMatchingMapper, selectors.All())
	}

	return best, nil
}

// mapperSpecificity returns the specificity score for a mapper.
//
// Specificity is defined as the number of selectors the mapper requires.
// Higher scores indicate more restrictive (specific) mappers.
//
// Example:
//   - Mapper with selectors [type:app] → score = 1
//   - Mapper with selectors [type:app, env:prod] → score = 2
//   - Mapper with selectors [type:app, env:prod, region:us] → score = 3
//
// The mapper with the highest score is considered most specific and will be
// selected when multiple mappers match a workload's selectors.
//
// This function is package-private and used by MatchWorkloadToMapper.
func mapperSpecificity(mapper *IdentityMapper) int {
	return len(mapper.Selectors().All())
}
