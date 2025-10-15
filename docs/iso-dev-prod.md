# Problem: Isolating Development-Only Code (Historical)

**Status**: Documented

Production binaries include ~760 lines of development-only domain code (selector matching, identity mapping, attestation logic) due to Go's type system constraints and hexagonal architecture design. While in-memory adapters are properly excluded via build tags, domain types and port interfaces cannot be isolated because struct fields and interface signatures force compile-time dependencies. Impact is minimal (~60KB/0.46% binary size) but creates conceptual pollution where unused code paths exist in production deployments.
