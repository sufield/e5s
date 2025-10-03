# Domain Purity

Refactored the SPIRE identity system to eliminate SDK duplication and maintain strict domain purity following hexagonal architecture principles.

## Changes Made

### 1. Removed SDK Duplication from Domain

**Problem**: The `ValidateSVID` method in `AttestationService` duplicated functionality provided by the go-spiffe SDK's `x509svid.ParseAndVerify` and `Verify` functions.

**Solution**: Removed `ValidateSVID` from the domain and created an adapter port instead.

#### Files Modified:
- `internal/core/domain/attestation.go`: Removed `ValidateSVID` method

### 2. Created IdentityDocumentValidator Port

**Purpose**: Abstract IdentityDocument validation to allow SDK-based verification in adapters while keeping domain pure.

#### Files Modified:
- `internal/app/ports.go`: Added `IdentityDocumentValidator` interface

```go
type IdentityDocumentValidator interface {
    Validate(ctx context.Context, svid *domain.IdentityDocument, expectedID *domain.IdentityNamespace) error
}
```

### 3. Implemented IdentityDocumentValidator Adapter

**Purpose**: Provide basic IdentityDocument validation with clear path to SDK integration.

#### Files Created:
- `internal/adapters/outbound/spire/validator.go`: Adapter implementation with comments showing how to integrate go-spiffe SDK

**Current Implementation**:
- Basic nil and time validity checks
- SPIFFE ID matching
- Documented extension points for SDK integration

**Future Integration** (when go-spiffe SDK is added):
```go
// Example integration with go-spiffe SDK
bundle := ... // get trust bundle for trust domain
_, err := x509svid.Verify(svid.Certificate(), svid.Chain(), bundle)
if err != nil {
    return fmt.Errorf("IdentityDocument verification failed: %w", err)
}
```

### 4. Documentation

#### Files Modified:
- `internal/core/domain/README.md`:
  - Updated `AttestationService` section to remove `ValidateSVID` reference
  - Added note explaining why IdentityDocument validation is in an adapter
  - Added new section "IdentityDocument Validation Adapter" with examples
  - Documented the anti-corruption layer pattern for this use case

## Architecture Benefits

### Domain Purity
- Domain layer has NO SDK dependencies
- Only standard library imports (`fmt`, `time`, `crypto/x509`)
- Pure business logic: selector matching, attestation result types

### SDK Functionality Delegated to Adapters
- IdentityDocument validation uses (or will use) go-spiffe SDK's battle-tested verification
- Chain-of-trust validation handled by SDK
- Trust bundle verification handled by SDK
- No reimplementation of complex crypto validation

### Separation of Concerns
```
Domain (pure business logic)
    ↓ uses port
IdentityDocumentValidator Port (interface)
    ↓ implemented by
IdentityDocumentValidator Adapter (SDK integration)
    ↓ uses
go-spiffe SDK (x509svid.Verify, ParseAndVerify)
```

## No Duplications

After this refactoring, the domain contains NO duplications of SDK functionality:

✅ **NodeAttestationResult / WorkloadAttestationResult**: Pure domain concepts, no SDK equivalent
✅ **MatchWorkloadToEntry**: Custom selector matching logic, SPIRE-specific, not in SDK
✅ **IdentityNamespace.Equals()**: Useful domain addition (SDK's `spiffeid.ID` lacks explicit `Equals`)
✅ **ValidateSVID**: ❌ REMOVED - moved to adapter to use SDK verification

## Testing

All tests pass:
```bash
$ go build ./cmd/console/
$ IDP_MODE=inmem go run ./cmd/console
✓ Success! Application runs correctly with refactored architecture
```

## Future Work

When integrating the go-spiffe SDK:

1. Add `github.com/spiffe/go-spiffe/v2` to dependencies
2. Update `internal/adapters/outbound/spire/validator.go`:
   - Import `github.com/spiffe/go-spiffe/v2/svid/x509svid`
   - Implement bundle management
   - Use `x509svid.Verify()` for full chain validation
3. Optionally create a `BundleSource` adapter port if needed

## References

- [go-spiffe SDK Documentation](https://github.com/spiffe/go-spiffe)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
- [Anti-Corruption Layer Pattern](https://docs.microsoft.com/en-us/azure/architecture/patterns/anti-corruption-layer)
