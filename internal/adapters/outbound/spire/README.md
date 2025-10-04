# Real SPIRE SDK Integration

This directory is reserved for real SPIRE implementations using the go-spiffe SDK. These adapters will replace in-memory implementations when deploying to production with real SPIRE infrastructure.

## Overview

The in-memory implementations in `../inmemory/` provide a walking skeleton that validates the architecture. This directory will contain production-ready adapters that:

1. Connect to real SPIRE server and agents
2. Use go-spiffe SDK for cryptographic operations
3. Implement full chain-of-trust verification
4. Support federation and policy enforcement

## Planned Adapters

### 1. TrustDomainParser (SDK Wrapper)

**Location**: `spire/trust_domain_parser.go`

**Purpose**: Parse and validate trust domain strings using SDK

**Implementation**:
```go
package spire

import (
    "context"
    "fmt"

    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/pocket/hexagon/spire/internal/domain"
    "github.com/pocket/hexagon/spire/internal/ports"
)

type SDKTrustDomainParser struct{}

func NewSDKTrustDomainParser() ports.TrustDomainParser {
    return &SDKTrustDomainParser{}
}

func (p *SDKTrustDomainParser) FromString(ctx context.Context, name string) (*domain.TrustDomain, error) {
    td, err := spiffeid.TrustDomainFromString(name)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTrustDomain, err)
    }
    // Convert SDK type to domain type
    domainTD, err := domain.NewTrustDomain(td.String())
    if err != nil {
        return nil, fmt.Errorf("%w: %v", domain.ErrInvalidTrustDomain, err)
    }
    return domainTD, nil
}
```

**Key Points**:
- Delegates validation to `spiffeid.TrustDomainFromString`
- Maps SDK errors to `domain.ErrInvalidTrustDomain`
- Ensures DNS label compliance

---

### 2. IdentityNamespaceParser (SDK Wrapper)

**Location**: `spire/identity_namespace_parser.go`

**Purpose**: Parse SPIFFE IDs using SDK validation

**Implementation**:
```go
type SDKIdentityNamespaceParser struct {
    tdParser ports.TrustDomainParser  // Injected for dependency management
}

func NewSDKIdentityNamespaceParser(tdParser ports.TrustDomainParser) ports.IdentityNamespaceParser {
    return &SDKIdentityNamespaceParser{tdParser: tdParser}
}

func (p *SDKIdentityNamespaceParser) ParseFromString(ctx context.Context, id string) (*domain.IdentityNamespace, error) {
    spiffeID, err := spiffeid.FromString(id)
    if err != nil {
        return nil, fmt.Errorf("%w: %v", domain.ErrInvalidIdentityNamespace, err)
    }

    td, err := p.tdParser.FromString(ctx, spiffeID.TrustDomain().String())
    if err != nil {
        return nil, err  // Propagate trust domain parsing error
    }

    return domain.NewIdentityNamespace(td, spiffeID.Path())
}

func (p *SDKIdentityNamespaceParser) ParseFromPath(ctx context.Context, trustDomain *domain.TrustDomain, path string) (*domain.IdentityNamespace, error) {
    if trustDomain == nil {
        return nil, fmt.Errorf("%w: trust domain cannot be nil", domain.ErrInvalidIdentityNamespace)
    }
    return domain.NewIdentityNamespace(trustDomain, path)
}
```

**Key Points**:
- Uses `spiffeid.FromString` for validation
- Extracts trust domain and path components
- Handles URI scheme validation

---

### 3. IdentityDocumentProvider (SDK X.509 SVID)

**Location**: `spire/identity_document_provider.go`

**Purpose**: Create and validate X.509 SVIDs using SDK

**Implementation**:
```go
import (
    "context"
    "crypto/rand"
    "crypto/rsa"
    "crypto/x509"
    "crypto/x509/pkix"
    "fmt"
    "math/big"
    "net/url"
    "time"

    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/svid/x509svid"
)

type SDKIdentityDocumentProvider struct {
    bundleProvider ports.TrustBundleProvider  // Injected for validation
}

func NewSDKIdentityDocumentProvider(bundleProvider ports.TrustBundleProvider) ports.IdentityDocumentProvider {
    return &SDKIdentityDocumentProvider{bundleProvider: bundleProvider}
}

func (p *SDKIdentityDocumentProvider) CreateX509IdentityDocument(
    ctx context.Context,
    identityNamespace *domain.IdentityNamespace,
    caCert interface{},
    caKey interface{},
) (*domain.IdentityDocument, error) {
    // Use SDK to generate X.509 certificate
    spiffeID, err := spiffeid.FromString(identityNamespace.String())
    if err != nil {
        return nil, fmt.Errorf("%w: invalid SPIFFE ID: %v", domain.ErrInvalidIdentityNamespace, err)
    }

    // Generate key pair
    privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
    if err != nil {
        return nil, fmt.Errorf("%w: key generation failed: %v", domain.ErrIdentityDocumentInvalid, err)
    }

    // Create certificate template with SPIFFE ID in URI SAN
    serialNumber, _ := rand.Int(rand.Reader, big.NewInt(1<<62))
    template := &x509.Certificate{
        SerialNumber: serialNumber,
        Subject:      pkix.Name{},  // SPIFFE uses URI SAN, not Subject
        NotBefore:    time.Now(),
        NotAfter:     time.Now().Add(24 * time.Hour),  // Short expiry for testing
        KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
        ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
        URIs:         []*url.URL{spiffeID.URL()},
        BasicConstraintsValid: true,
        IsCA: false,
    }

    // Sign with CA
    certDER, err := x509.CreateCertificate(rand.Reader, template, caCert.(*x509.Certificate),
        &privateKey.PublicKey, caKey)
    if err != nil {
        return nil, fmt.Errorf("%w: certificate creation failed: %v", domain.ErrIdentityDocumentInvalid, err)
    }

    // Parse and return as domain type
    cert, err := x509.ParseCertificate(certDER)
    if err != nil {
        return nil, fmt.Errorf("%w: certificate parsing failed: %v", domain.ErrIdentityDocumentInvalid, err)
    }

    return domain.NewIdentityDocument(identityNamespace, cert, privateKey)
}

func (p *SDKIdentityDocumentProvider) ValidateIdentityDocument(
    ctx context.Context,
    doc *domain.IdentityDocument,
    expectedID *domain.IdentityNamespace,
) error {
    // Use bundleProvider (injected) to get trust bundle
    bundle, err := p.bundleProvider.GetBundleForIdentity(ctx, doc.IdentityNamespace())
    if err != nil {
        return fmt.Errorf("%w: bundle not found", domain.ErrCertificateChainInvalid)
    }

    // Parse bundle
    bundleSource, err := x509bundle.Parse(doc.IdentityNamespace().TrustDomain(), bundle)
    if err != nil {
        return fmt.Errorf("%w: bundle parse failed", domain.ErrCertificateChainInvalid)
    }

    // Build full chain
    fullChain := append([]*x509.Certificate{doc.Certificate()}, doc.Chain()...)

    // Verify using SDK
    spiffeID, _, err := x509svid.Verify(fullChain, bundleSource)
    if err != nil {
        return fmt.Errorf("%w: %v", domain.ErrCertificateChainInvalid, err)
    }

    // Validate SPIFFE ID match
    if spiffeID.String() != expectedID.String() {
        return fmt.Errorf("%w", domain.ErrIdentityDocumentMismatch)
    }

    return nil
}
```

---

### 4. TrustBundleProvider (SDK Bundle Source)

**Location**: `spire/trust_bundle_provider.go`

**Purpose**: Fetch trust bundles from SPIRE server

**Implementation**:
```go
import (
    "context"
    "fmt"

    "github.com/spiffe/go-spiffe/v2/bundle/x509bundle"
    "github.com/spiffe/go-spiffe/v2/spiffeid"
    "github.com/spiffe/go-spiffe/v2/workloadapi"
)

type SDKTrustBundleProvider struct {
    client *workloadapi.Client
}

func NewSDKTrustBundleProvider(ctx context.Context, socketPath string) (ports.TrustBundleProvider, error) {
    client, err := workloadapi.New(ctx, workloadapi.WithAddr(socketPath))
    if err != nil {
        return nil, fmt.Errorf("failed to create workload API client: %w", err)
    }
    return &SDKTrustBundleProvider{client: client}, nil
}

func (p *SDKTrustBundleProvider) GetBundle(ctx context.Context, trustDomain *domain.TrustDomain) ([]byte, error) {
    if trustDomain == nil {
        return nil, fmt.Errorf("%w: trust domain cannot be nil", domain.ErrInvalidTrustDomain)
    }

    // Fetch bundle from Workload API
    td, err := spiffeid.TrustDomainFromString(trustDomain.String())
    if err != nil {
        return nil, fmt.Errorf("%w: invalid trust domain: %v", domain.ErrInvalidTrustDomain, err)
    }

    bundles, err := p.client.FetchX509Bundles(ctx)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to fetch bundles: %v", domain.ErrTrustBundleNotFound, err)
    }

    bundle, ok := bundles.Get(td)
    if !ok {
        return nil, fmt.Errorf("%w: for trust domain %s", domain.ErrTrustBundleNotFound, trustDomain.String())
    }

    // Marshal bundle to PEM (port contract requires []byte)
    pemBytes, err := bundle.Marshal()
    if err != nil {
        return nil, fmt.Errorf("%w: failed to marshal bundle: %v", domain.ErrTrustBundleNotFound, err)
    }

    return pemBytes, nil
}

func (p *SDKTrustBundleProvider) GetBundleForIdentity(ctx context.Context, identityNamespace *domain.IdentityNamespace) ([]byte, error) {
    if identityNamespace == nil {
        return nil, fmt.Errorf("%w: identity namespace cannot be nil", domain.ErrInvalidIdentityNamespace)
    }
    return p.GetBundle(ctx, identityNamespace.TrustDomain())
}
```

**Key Points**:
- Connects to Workload API for bundle fetching
- Supports federation (multiple trust domains)
- Returns PEM-encoded bundle (port contract)

---

### 5. Server Adapter (Workload API Integration)

**Location**: `spire/server.go`

**Purpose**: Issue SVIDs via SPIRE server

**Note**: In production, the "server" functionality is accessed via Workload API. The agent mediates between workloads and server. This adapter would:

1. Connect to SPIRE server's registration API (for admin operations)
2. Or delegate to agent's Workload API (for SVID issuance)

**Implementation Strategy**:
- For SVID issuance: Use agent's Workload API (same as workloads)
- For registration: Use SPIRE server's gRPC API with admin credentials
- May split into `ServerRegistrationAdapter` (admin) and `ServerSVIDAdapter` (via agent)

---

### 6. Agent Adapter (Workload API Client)

**Location**: `spire/agent.go`

**Purpose**: Fetch SVIDs from SPIRE agent via Workload API

**Implementation**:
```go
import (
    "context"
    "fmt"
    "sync"

    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "github.com/pocket/hexagon/spire/internal/domain"
    "github.com/pocket/hexagon/spire/internal/ports"
)

type SPIREAgent struct {
    clientOnce    sync.Once
    client        *workloadapi.Client
    clientErr     error
    socketPath    string
    parser        ports.IdentityNamespaceParser  // Injected for domain type conversion
    agentIdentity *ports.Identity
}

func NewSPIREAgent(ctx context.Context, socketPath string, parser ports.IdentityNamespaceParser) (ports.Agent, error) {
    if socketPath == "" {
        return nil, fmt.Errorf("%w: socket path cannot be empty", domain.ErrAgentUnavailable)
    }
    if parser == nil {
        return nil, fmt.Errorf("%w: parser cannot be nil", domain.ErrInvalidDependency)
    }

    return &SPIREAgent{
        socketPath: socketPath,
        parser:     parser,
    }, nil
}

// getClient lazily initializes the Workload API client with sync.Once for reliability
// This pattern ensures connection failures don't prevent service startup
func (a *SPIREAgent) getClient(ctx context.Context) (*workloadapi.Client, error) {
    a.clientOnce.Do(func() {
        // Lazy connection with custom backoff (go-spiffe v2.6.0+)
        a.client, a.clientErr = workloadapi.New(ctx,
            workloadapi.WithAddr(a.socketPath),
            // Optional: workloadapi.WithClientOptions(...) for custom retry/timeout
        )
    })

    if a.clientErr != nil {
        return nil, fmt.Errorf("%w: failed to create workload API client: %v", domain.ErrAgentUnavailable, a.clientErr)
    }

    return a.client, nil
}

func (a *SPIREAgent) GetIdentity(ctx context.Context) (*ports.Identity, error) {
    if a.agentIdentity == nil {
        return nil, fmt.Errorf("%w: agent identity not initialized", domain.ErrAgentUnavailable)
    }
    return a.agentIdentity, nil
}

func (a *SPIREAgent) FetchIdentityDocument(ctx context.Context, workload ports.ProcessIdentity) (*ports.Identity, error) {
    // NOTE: In real SPIRE, the agent extracts workload identity from Unix socket credentials
    // The Workload API client doesn't pass process info - the server extracts it automatically
    // The ProcessIdentity parameter is ignored in this implementation (for port compatibility)

    // Get client (lazy init)
    client, err := a.getClient(ctx)
    if err != nil {
        return nil, err  // Error already wrapped in getClient
    }

    // Fetch SVID from agent
    svid, err := client.FetchX509SVID(ctx)
    if err != nil {
        // Map SDK errors to domain errors
        return nil, mapSDKError(err)
    }

    // Convert SDK SVID to domain types
    identityNamespace, err := a.parser.ParseFromString(ctx, svid.ID.String())
    if err != nil {
        return nil, fmt.Errorf("%w: failed to parse SPIFFE ID: %v", domain.ErrInvalidIdentityNamespace, err)
    }

    // Build identity document with full chain
    // svid.Certificates[0] is leaf, svid.Certificates[1:] are intermediates
    doc, err := domain.NewIdentityDocument(
        identityNamespace,
        svid.Certificates[0],      // Leaf certificate
        svid.PrivateKey,            // Private key
        svid.Certificates[1:]...,   // Intermediate certificates (chain)
    )
    if err != nil {
        return nil, fmt.Errorf("%w: failed to create identity document: %v", domain.ErrIdentityDocumentInvalid, err)
    }

    return &ports.Identity{
        IdentityNamespace: identityNamespace,
        IdentityDocument:  doc,
    }, nil
}

// HealthCheck verifies connection to SPIRE agent (for readiness probes)
func (a *SPIREAgent) HealthCheck(ctx context.Context) error {
    client, err := a.getClient(ctx)
    if err != nil {
        return err
    }

    // Attempt lightweight operation to verify connection
    _, err = client.FetchX509Bundles(ctx)
    if err != nil {
        return fmt.Errorf("%w: bundle fetch failed: %v", domain.ErrAgentUnavailable, err)
    }

    return nil
}
```

**Key Points**:
- Uses `workloadapi.Client.FetchX509SVID()` (matches our port signature)
- Agent automatically attests calling workload via Unix socket credentials
- No need to pass `ProcessIdentity` - server extracts it
- For this implementation, the port's `ProcessIdentity` parameter is ignored (attestation is automatic)
- Lazy client initialization with `sync.Once` for reliability
- Health check method for readiness probes

---

### Error Mapping Helper (Domain Purity)

**Purpose**: Map SDK errors to typed domain errors

SDK errors leak implementation details into the domain layer. This helper maps SDK error types to domain sentinel errors for clean error handling:

```go
import (
    "errors"
    "fmt"

    "github.com/spiffe/go-spiffe/v2/workloadapi"
    "github.com/pocket/hexagon/spire/internal/domain"
)

// mapSDKError maps go-spiffe SDK errors to domain errors
// This maintains domain purity by preventing SDK error types from leaking
func mapSDKError(err error) error {
    if err == nil {
        return nil
    }

    // Connection errors -> ErrAgentUnavailable
    if errors.Is(err, workloadapi.ErrNoConnection) {
        return fmt.Errorf("%w: agent connection failed: %v", domain.ErrAgentUnavailable, err)
    }

    // Authorization errors -> ErrWorkloadAttestationFailed
    if errors.Is(err, workloadapi.ErrNotAuthorized) {
        return fmt.Errorf("%w: workload not authorized: %v", domain.ErrWorkloadAttestationFailed, err)
    }

    // SVID fetch errors -> ErrIdentityDocumentInvalid
    if errors.Is(err, workloadapi.ErrNoSVID) {
        return fmt.Errorf("%w: no SVID available: %v", domain.ErrIdentityDocumentInvalid, err)
    }

    // Bundle errors -> ErrTrustBundleNotFound
    if errors.Is(err, workloadapi.ErrNoBundle) {
        return fmt.Errorf("%w: no bundle available: %v", domain.ErrTrustBundleNotFound, err)
    }

    // Default: wrap as generic attestation failure
    return fmt.Errorf("%w: SDK operation failed: %v", domain.ErrWorkloadAttestationFailed, err)
}
```

**Usage Example**:
```go
svid, err := client.FetchX509SVID(ctx)
if err != nil {
    return nil, mapSDKError(err)  // Maps to domain error
}
```

**Benefits**:
- Application code handles `domain.ErrAgentUnavailable`, not `workloadapi.ErrNoConnection`
- Domain layer remains SDK-agnostic
- Clear error semantics for business logic
- Easy to swap SDK without changing error handling

---

### 7. WorkloadAttestor (Real Unix Attestation)

**Location**: `spire/workload_attestor.go`

**Purpose**: Attest workloads using real platform mechanisms

**Unix Implementation**:
```go
import (
    "context"
    "fmt"
    "os"
)

type UnixWorkloadAttestor struct{}

func NewUnixWorkloadAttestor() ports.WorkloadAttestor {
    return &UnixWorkloadAttestor{}
}

func (a *UnixWorkloadAttestor) Attest(ctx context.Context, workload ports.ProcessIdentity) ([]string, error) {
    // Validate input
    if workload.PID <= 0 {
        return nil, fmt.Errorf("%w: invalid PID %d", domain.ErrInvalidProcessIdentity, workload.PID)
    }

    // Read /proc filesystem for real attestation
    procPath := fmt.Sprintf("/proc/%d", workload.PID)

    // Extract binary path
    exePath, err := os.Readlink(fmt.Sprintf("%s/exe", procPath))
    if err != nil {
        return nil, fmt.Errorf("%w: cannot read process exe: %v", domain.ErrWorkloadAttestationFailed, err)
    }

    // Read process status for additional attributes (optional)
    statusPath := fmt.Sprintf("%s/status", procPath)
    _, err = os.ReadFile(statusPath)
    if err != nil {
        return nil, fmt.Errorf("%w: cannot read process status: %v", domain.ErrWorkloadAttestationFailed, err)
    }

    // Generate selectors
    selectors := []string{
        fmt.Sprintf("unix:uid:%d", workload.UID),
        fmt.Sprintf("unix:gid:%d", workload.GID),
        fmt.Sprintf("unix:path:%s", exePath),
        // Could add: unix:sha256:<hash>, unix:user:<username>, etc.
    }

    if len(selectors) == 0 {
        return nil, fmt.Errorf("%w: no selectors generated", domain.ErrNoAttestationData)
    }

    return selectors, nil
}
```

**Other Attestors**:
- **AWS**: Verify EC2 instance identity document, extract instance ID, region, tags
- **GCP**: Verify GCE instance identity token, extract project, zone, labels
- **Kubernetes**: Verify pod service account token, extract namespace, service account, pod name
- **Azure**: Verify managed identity token

---

## Migration Path

### Phase 1: Setup Real SPIRE Infrastructure

1. **Deploy SPIRE Server**:
   ```bash
   spire-server run -config server.conf
   ```

2. **Deploy SPIRE Agent**:
   ```bash
   spire-agent run -config agent.conf -socketPath /tmp/spire-agent/public/api.sock
   ```

3. **Register Agent with Server**:
   ```bash
   # Note: Use -node flag to distinguish agent entries from workload entries
   spire-server entry create \
     -spiffeID spiffe://example.org/agent \
     -parentID spiffe://example.org/spire/server \
     -selector type:join_token \
     -node
   ```

4. **Register Workloads**:
   ```bash
   spire-server entry create \
     -spiffeID spiffe://example.org/server-workload \
     -parentID spiffe://example.org/agent \
     -selector unix:uid:1001
   ```

### Phase 2: Implement SDK Adapters

Create adapters in this directory following the patterns above:

1. **Start with parsers** (low risk, pure validation):
   - `trust_domain_parser.go`
   - `identity_namespace_parser.go`

2. **Add bundle and validation**:
   - `trust_bundle_provider.go`
   - `identity_document_provider.go` (validation part)

3. **Integrate with agent**:
   - `agent.go` (Workload API client)

4. **Add document generation** (if needed):
   - `identity_document_provider.go` (creation part)
   - Note: In production, SPIRE server generates SVIDs, not the application

### Phase 3: Create Real Dependency Factory

**Location**: `internal/adapters/outbound/compose/spire.go`

```go
package compose

import (
    "github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
    "github.com/pocket/hexagon/spire/internal/ports"
)

type SPIREDeps struct {
    socketPath string
}

func NewSPIREDeps(socketPath string) *SPIREDeps {
    return &SPIREDeps{socketPath: socketPath}
}

func (d *SPIREDeps) CreateTrustDomainParser() ports.TrustDomainParser {
    return spire.NewSDKTrustDomainParser()
}

func (d *SPIREDeps) CreateIdentityNamespaceParser() ports.IdentityNamespaceParser {
    return spire.NewSDKIdentityNamespaceParser()
}

func (d *SPIREDeps) CreateTrustBundleProvider(server ports.Server) ports.TrustBundleProvider {
    provider, _ := spire.NewSDKTrustBundleProvider(d.socketPath)
    return provider
}

func (d *SPIREDeps) CreateAgent(/* ... */) (ports.Agent, error) {
    return spire.NewSPIREAgent(d.socketPath)
}

// ... other factory methods
```

### Phase 4: Switch Dependency Injection

**In `cmd/agent/main.go`**:

```go
var deps compose.Dependencies

mode := os.Getenv("IDP_MODE")
switch mode {
case "spire":
    socketPath := os.Getenv("SPIRE_AGENT_SOCKET")
    if socketPath == "" {
        socketPath = "/tmp/spire-agent/public/api.sock"
    }
    deps = compose.NewSPIREDeps(socketPath)
case "inmem":
    deps = compose.NewInMemoryDeps()
default:
    log.Fatal("IDP_MODE must be 'inmem' or 'spire'")
}

app, err := app.Bootstrap(ctx, configLoader, deps)
```

### Phase 5: Test Migration

1. **Run with in-memory**:
   ```bash
   IDP_MODE=inmem ./bin/agent
   ```

2. **Run with real SPIRE**:
   ```bash
   IDP_MODE=spire SPIRE_AGENT_SOCKET=/tmp/spire-agent/public/api.sock ./bin/agent
   ```

3. **Verify workload fetch**:
   ```bash
   IDP_MODE=spire ./bin/workload
   ```

---

## Dependencies

Add to `go.mod`:

```go
require (
    github.com/spiffe/go-spiffe/v2 v2.6.0      // Latest: Go 1.24+, Ed25519, empty bundles
    github.com/spiffe/spire-api-sdk/proto/spire/api/server v1.13.0  // SPIRE v1.13.0 compatible
)
```

**Version Notes**:
- `go-spiffe/v2 v2.6.0`: Released Aug 21, 2025 with Go 1.24+ support, Ed25519 keys, custom backoff
- `spire-api-sdk v1.13.1`: Released Oct 4, 2025 aligned with SPIRE v1.13.1
- No breaking changes from earlier examples in this document
- Check [SPIRE Releases](https://github.com/spiffe/spire/releases) for updates, as api-sdk tags align with SPIRE versions

## Testing Strategy

### Unit Tests

Mock SDK interfaces for isolated testing:

```go
type MockWorkloadAPIClient struct {
    mock.Mock
}

func (m *MockWorkloadAPIClient) FetchX509SVID(ctx context.Context) (*x509svid.SVID, error) {
    args := m.Called(ctx)
    return args.Get(0).(*x509svid.SVID), args.Error(1)
}

func TestSPIREAgent_FetchIdentityDocument(t *testing.T) {
    mockClient := &MockWorkloadAPIClient{}
    mockSVID := &x509svid.SVID{/* ... */}
    mockClient.On("FetchX509SVID", mock.Anything).Return(mockSVID, nil)

    agent := &SPIREAgent{client: mockClient}
    identity, err := agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{})

    require.NoError(t, err)
    assert.Equal(t, "spiffe://example.org/workload", identity.IdentityNamespace.String())
}
```

### Integration Tests

Test against real SPIRE in CI:

```go
func TestRealSPIREIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("skipping integration test")
    }

    // Start SPIRE server and agent in Docker
    compose := testcontainers.NewLocalDockerCompose(/* ... */)
    defer compose.Down()

    // Create SPIRE deps
    deps := compose.NewSPIREDeps("/tmp/spire-agent/public/api.sock")
    app, _ := app.Bootstrap(ctx, configLoader, deps)

    // Test SVID fetch
    identity, err := app.Agent.FetchIdentityDocument(ctx, ports.ProcessIdentity{UID: 1000})
    require.NoError(t, err)
    assert.NotNil(t, identity.IdentityDocument)
}
```

---

## Security Considerations

1. **Socket Permissions**: Ensure `/tmp/spire-agent/public/api.sock` has correct permissions (0600)
2. **mTLS**: Enable mTLS between agent and server in production
3. **Rotation**: SVIDs auto-rotate before expiry (SDK handles this)
4. **Attestation**: Use multiple selectors for defense-in-depth
5. **Federation**: Configure trust bundles for cross-domain trust

## References

- [go-spiffe SDK Documentation](https://github.com/spiffe/go-spiffe)
- [SPIRE Agent Configuration](https://spiffe.io/docs/latest/deploying/spire_agent/)
- [SPIRE Server Configuration](https://spiffe.io/docs/latest/deploying/spire_server/)
- [Workload API Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Workload_API.md)
- [SPIFFE ID Specification](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE-ID.md)
