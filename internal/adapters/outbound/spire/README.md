# Real SPIRE Implementation (Future)

This directory is reserved for real SPIRE implementations using the go-spiffe SDK.

## Planned Adapters

### Server Adapter
- Use `spiffe.io/go-spiffe/v2/spiffeid` for identity parsing
- Use `spiffe.io/go-spiffe/v2/bundle/x509bundle` for trust bundle management
- Use `spiffe.io/go-spiffe/v2/svid/x509svid` for X.509 IdentityDocument operations
- Connect to real SPIRE server via Workload API

### Agent Adapter
- Use `spiffe.io/go-spiffe/v2/workloadapi` for fetching SVIDs
- Use `spiffe.io/go-spiffe/v2/svid/x509svid` for IdentityDocument validation
- Connect to SPIRE agent socket (typically `/tmp/spire-agent/public/api.sock`)

### Store Adapter
- Connect to SPIRE server registration API
- Use gRPC for server communication
- Implement proper error handling and retries

### Attestor Adapters
- **Unix Attestor**: Real Unix process attestation using `/proc` filesystem
- **AWS Attestor**: EC2 instance identity document verification
- **GCP Attestor**: GCE instance identity token verification
- **Azure Attestor**: Managed Service Identity verification
- **Kubernetes Attestor**: Pod identity attestation

## Migration Path

To migrate from in-memory to real SPIRE:

1. Implement adapters in this directory
2. Update `compose/inmemory.go` to create a new factory (e.g., `RealSpireDeps`)
3. Switch dependency factory in `cmd/console/main.go`
4. All ports remain unchanged - hexagonal architecture ensures compatibility

## Dependencies

```go
require (
    github.com/spiffe/go-spiffe/v2 v2.x.x
    github.com/spiffe/spire-api-sdk/proto/spire/api/server/... vX.X.X
)
```

## References

- [go-spiffe SDK Documentation](https://github.com/spiffe/go-spiffe)
- [SPIRE Agent API](https://github.com/spiffe/spire-api-sdk)
- [Workload API Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_API.md)
