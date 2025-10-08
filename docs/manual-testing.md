# Port-Based mTLS Improvements

Port-based improvements were made to the mTLS implementation, following proper separation of concerns.

### 1. Port Interfaces (internal/ports/identityserver.go)

Created port interfaces that separate configuration from behavior:

**Before**: Configuration and behavior were mixed in adapter implementations
**After**: Clear separation with port interfaces

```go
// Configuration (pure data)
type ServerConfig struct {
    WorkloadAPI WorkloadAPIConfig
    SPIFFE      SPIFFEServerConfig
    HTTP        HTTPServerConfig
}

// Behavior (port interface)
type MTLSServer interface {
    Handle(pattern string, handler http.Handler)
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
    Close() error
}
```

### 2. Improved Adapter Implementation (internal/adapters/inbound/identityserver/)

**New Features**:
- ✅ Implements `ports.MTLSServer` interface
- ✅ Proper timeout configuration (ReadHeaderTimeout, WriteTimeout, IdleTimeout)
- ✅ Prevents Slowloris attacks with ReadHeaderTimeout
- ✅ Idempotent Close() and Shutdown() operations
- ✅ Thread-safe with mutex protection
- ✅ Better error handling and logging
- ✅ Graceful shutdown with context cancellation

**Security Improvements**:

```go
HTTP: ports.HTTPServerConfig{
    ReadHeaderTimeout: 10 * time.Second,  // Prevents Slowloris attacks
    WriteTimeout:      30 * time.Second,  // Prevents slow writes
    IdleTimeout:       120 * time.Second, // Closes idle connections
}
```

### 3. Enhanced Configuration (internal/config/mtls.go)

**New Conversion Methods**:

```go
// Convert YAML config to port config
func (c *MTLSConfig) ToServerConfig() ports.ServerConfig
func (c *MTLSConfig) ToClientConfig() ports.ClientConfig
```

This allows seamless integration between YAML configuration and port-based adapters.

### 4. Improved Example (examples/identityserver-example/)

**Features**:

- Environment-based configuration
- Proper signal handling (SIGINT, SIGTERM)
- Graceful shutdown
- Multiple endpoints demonstrating different features
- Separation of concerns
- Uses port interfaces

```go
server, err := identityserver.NewSPIFFEServer(ctx, cfg)
if err != nil {
    log.Fatalf("Failed to create server: %v", err)
}
defer server.Close()

server.Handle("/api/hello", http.HandlerFunc(handleHello))
server.Start(ctx)
```

## Benefits

### Architecture Compliance

```
┌─────────────────────────────────────────────────────────┐
│                  Application Layer                       │
│              (Business Logic)                            │
│                                                          │
│  Uses: ports.MTLSServer (interface)                     │
│  Does NOT depend on: specific implementations           │
└──────────────────┬──────────────────────────────────────┘
                   │
                   │ Depends on abstraction (port)
                   ▼
┌─────────────────────────────────────────────────────────┐
│               Port Interfaces (Stable)                   │
│                                                          │
│  type MTLSServer interface {                            │
│      Handle(pattern string, handler http.Handler)       │
│      Start(ctx context.Context) error                   │
│      ...                                                 │
│  }                                                       │
└──────────────────┬──────────────────────────────────────┘
                   ▲
                   │ Implements abstraction
                   │
┌─────────────────────────────────────────────────────────┐
│            Infrastructure Adapters                       │
│                                                          │
│  identityserver.NewSPIFFEServer()                       │
│  - Uses go-spiffe SDK                                   │
│  - Implements ports.MTLSServer                          │
│  - Can be swapped with different implementation         │
└─────────────────────────────────────────────────────────┘
```

1. **Dependency Inversion**: Application depends on abstractions (ports), not concrete implementations
2. **Testability**: Easy to mock `ports.MTLSServer` for testing
3. **Flexibility**: Can swap SPIFFE implementation without changing application code
4. **Configuration Separation**: Pure data structures (Config) separate from behavior (Server)
5. **Clear Contracts**: Port interfaces define clear contracts between layers

## Comparison

### Before (httpapi adapter)

```go
// Mixed configuration and behavior
server, err := httpapi.NewHTTPServer(
    ctx,
    ":8443",                    // inline string
    "unix:///tmp/socket",       // inline string
    tlsconfig.AuthorizeAny(),   // inline config
)
```

**Issues**:

- Configuration scattered in function call
- No separation of configuration from behavior
- No type safety for configuration
- Hard to test different configurations

### After (port-based)

```go
// Configuration as data structure
var cfg ports.ServerConfig
cfg.WorkloadAPI.SocketPath = "unix:///tmp/socket"
cfg.SPIFFE.AllowedClientID = "spiffe://example.org/client"
cfg.HTTP.Address = ":8443"
cfg.HTTP.ReadHeaderTimeout = 10 * time.Second

// Create server using configuration
server, err := identityserver.NewSPIFFEServer(ctx, cfg)
```

**Benefits**:

- Configuration is a first-class data structure
- Type-safe and well-documented
- Easy to load from files, environment, or other sources
- Clear separation of concerns
- Better security with timeout configuration

## Files Changed/Created

| File | Type | Purpose |
|------|------|---------|
| [internal/ports/identityserver.go](../internal/ports/identityserver.go) | NEW | Port interfaces and config structs |
| [internal/adapters/inbound/identityserver/spiffe_server.go](../internal/adapters/inbound/identityserver/spiffe_server.go) | NEW | SPIFFE adapter implementing ports.MTLSServer |
| [internal/adapters/inbound/identityserver/spiffe_server_test.go](../internal/adapters/inbound/identityserver/spiffe_server_test.go) | NEW | Comprehensive tests |
| [internal/config/mtls.go](../internal/config/mtls.go) | MODIFIED | Added ToServerConfig() and ToClientConfig() |
| [examples/identityserver-example/main.go](../examples/identityserver-example/main.go) | NEW | Improved example using ports |

## Usage Examples

### Server with Port Interface

```go
import (
    "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
    "github.com/pocket/hexagon/spire/internal/ports"
)

func main() {
    ctx := context.Background()

    // Configuration
    var cfg ports.ServerConfig
    cfg.WorkloadAPI.SocketPath = os.Getenv("SPIRE_AGENT_SOCKET")
    cfg.SPIFFE.AllowedClientID = os.Getenv("ALLOWED_CLIENT_ID")
    cfg.HTTP.Address = ":8443"
    cfg.HTTP.ReadHeaderTimeout = 10 * time.Second

    // Create server (adapter)
    server, err := identityserver.NewSPIFFEServer(ctx, cfg)
    if err != nil {
        log.Fatal(err)
    }
    defer server.Close()

    // Register handlers
    server.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        clientID, _ := identityserver.GetSPIFFEID(r)
        fmt.Fprintf(w, "Hello, %s!\n", clientID)
    }))

    // Start server
    server.Start(ctx)
}
```

### With YAML Configuration

```go
import (
    "github.com/pocket/hexagon/spire/internal/config"
    "github.com/pocket/hexagon/spire/internal/adapters/inbound/identityserver"
)

func main() {
    // Load from YAML
    cfg, err := config.LoadFromFile("config.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // Convert to port config
    serverCfg := cfg.ToServerConfig()

    // Create server
    server, err := identityserver.NewSPIFFEServer(ctx, serverCfg)
    // ...
}
```

### Testing with Mock

```go
type MockMTLSServer struct {
    HandleFunc   func(pattern string, handler http.Handler)
    StartFunc    func(ctx context.Context) error
    ShutdownFunc func(ctx context.Context) error
    CloseFunc    func() error
}

func (m *MockMTLSServer) Handle(pattern string, handler http.Handler) {
    if m.HandleFunc != nil {
        m.HandleFunc(pattern, handler)
    }
}

func TestMyService(t *testing.T) {
    mockServer := &MockMTLSServer{
        StartFunc: func(ctx context.Context) error {
            return nil
        },
    }

    service := NewMyService(mockServer)
    // Test service without real SPIRE
}
```

## Security Improvements

### 1. Slowloris Attack Prevention

ReadHeaderTimeout prevents clients from slowly sending headers:

```go
cfg.HTTP.ReadHeaderTimeout = 10 * time.Second
```

Without this, attackers can hold connections open indefinitely.

### 2. Write Timeout

WriteTimeout prevents slow response writes:

```go
cfg.HTTP.WriteTimeout = 30 * time.Second
```

### 3. Idle Timeout

IdleTimeout closes idle connections:

```go
cfg.HTTP.IdleTimeout = 120 * time.Second
```

### 4. Thread Safety

Mutex protection for Close() and Shutdown():

```go
func (s *spiffeServer) Close() error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if s.closed {
        return nil  // Idempotent
    }
    s.closed = true
    // ...
}
```

## Migration Guide

### From httpapi to identityserver

**Old code**:
```go
server, err := httpapi.NewHTTPServer(ctx, ":8443", socket, authorizer)
server.RegisterHandler("/api", handler)
server.Start(ctx)
```

**New code**:
```go
var cfg ports.ServerConfig
cfg.WorkloadAPI.SocketPath = socket
cfg.SPIFFE.AllowedClientID = clientID
cfg.HTTP.Address = ":8443"

server, err := identityserver.NewSPIFFEServer(ctx, cfg)
server.Handle("/api", handler)
server.Start(ctx)
```

### Benefits of Migration

1. Better security (timeouts configured)
2. Cleaner configuration
3. Type-safe configuration structs
4. Easier testing with mocks
5. Better alignment with clean architecture

## Best Practices

### 1. Always Set Timeouts

```go
cfg.HTTP.ReadHeaderTimeout = 10 * time.Second  // REQUIRED for security
cfg.HTTP.WriteTimeout = 30 * time.Second       // Recommended
cfg.HTTP.IdleTimeout = 120 * time.Second       // Recommended
```

### 2. Use Environment Variables for Sensitive Config

```go
cfg.WorkloadAPI.SocketPath = os.Getenv("SPIRE_AGENT_SOCKET")
cfg.SPIFFE.AllowedClientID = os.Getenv("ALLOWED_CLIENT_ID")
```

### 3. Always Defer Close()

```go
server, err := identityserver.NewSPIFFEServer(ctx, cfg)
if err != nil {
    log.Fatal(err)
}
defer server.Close()  // IMPORTANT: Release resources
```

### 4. Handle Graceful Shutdown

```go
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

go func() {
    <-sigCh
    cancel()  // Cancel context to trigger graceful shutdown
}()

server.Start(ctx)  // Blocks until context cancelled
```

## References

- [Port Interfaces](../internal/ports/identityserver.go)
- [SPIFFE Server Adapter](../internal/adapters/inbound/identityserver/spiffe_server.go)
- [Improved Example](../examples/identityserver-example/main.go)
- [Configuration](../internal/config/mtls.go)
- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)
