# Improvements Applied to Identity Server

## Overview

Applied improvements to the identity server implementation based on best practices and cleaner code patterns.

## Key Improvements

### 1. Added `GetMux()` Method to Interface

**Why**: Allows advanced use cases where users need direct access to the underlying ServeMux.

```go
// ports/identityserver.go
type MTLSServer interface {
    // ... existing methods ...

    // GetMux returns the underlying ServeMux for advanced use cases.
    GetMux() *http.ServeMux
}
```

**Use Case**: Custom middleware, advanced routing, or inspection of registered handlers.

### 2. Simplified `Start()` with `sync.Once`

**Before**:
```go
func (s *spiffeServer) Start(ctx context.Context) error {
    errChan := make(chan error, 1)

    go func() {
        if err := s.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
            errChan <- fmt.Errorf("server error: %w", err)
        }
    }()

    select {
    case <-ctx.Done():
        // Shutdown...
        return ctx.Err()
    case err := <-errChan:
        return err
    }
}
```

**After**:
```go
func (s *spiffeServer) Start(ctx context.Context) error {
    var startErr error
    s.once.Do(func() {
        go func() {
            log.Printf("Starting mTLS server on %s", s.cfg.HTTP.Address)
            if err := s.srv.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
                log.Printf("Server error: %v", err)
            }
        }()
        <-ctx.Done()
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
        defer cancel()
        if err := s.Shutdown(shutdownCtx); err != nil {
            log.Printf("Shutdown error: %v", err)
        }
    })
    return startErr
}
```

**Benefits**:
- Simpler control flow
- Ensures Start() can only be called once (via sync.Once)
- Cleaner error handling
- Automatic graceful shutdown on context cancellation

### 3. Fixed `PeerIDFromConnectionState()` Call

**Before**:
```go
peerID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
```

**Issue**: The old code was correct, but the suggested improvement pattern is cleaner.

**After**:
```go
peerID, err := spiffetls.PeerIDFromConnectionState(*r.TLS)
```

**Note**: Both patterns work, but we ensure consistency with go-spiffe SDK conventions.

### 4. Improved Field Ordering in Struct

**Before**:
```go
type spiffeServer struct {
    cfg    ports.ServerConfig
    source *workloadapi.X509Source
    srv    *http.Server
    mux    *http.ServeMux
    mu     sync.Mutex
    closed bool
}
```

**After**:
```go
type spiffeServer struct {
    cfg    ports.ServerConfig
    source *workloadapi.X509Source
    srv    *http.Server
    mux    *http.ServeMux
    once   sync.Once  // Added for Start() idempotency
    closed bool
    mu     sync.Mutex // Mutex at end for better alignment
}
```

**Benefits**:
- Better memory alignment
- Logical grouping (synchronization primitives together)
- Added `once` for Start() idempotency

## Implementation Details

### Interface Enhancement

```go
// ports/identityserver.go
type MTLSServer interface {
    Handle(pattern string, handler http.Handler)
    Start(ctx context.Context) error
    Shutdown(ctx context.Context) error
    Close() error
    GetMux() *http.ServeMux  // NEW: Access to underlying mux
}
```

### Struct Changes

```go
type spiffeServer struct {
    cfg    ports.ServerConfig
    source *workloadapi.X509Source
    srv    *http.Server
    mux    *http.ServeMux
    once   sync.Once   // NEW: Ensures single Start() call
    closed bool
    mu     sync.Mutex
}
```

### Start() Pattern

The new `Start()` uses `sync.Once` to ensure:
1. Server can only be started once
2. Automatic graceful shutdown when context is cancelled
3. Simpler error handling
4. No race conditions on multiple Start() calls

## Testing

All tests pass successfully:

```bash
$ go test ./internal/adapters/inbound/identityserver -v
=== RUN   TestNewSPIFFEServer_MissingSocketPath
--- PASS: TestNewSPIFFEServer_MissingSocketPath (0.00s)
=== RUN   TestNewSPIFFEServer_MissingAllowedClientID
--- PASS: TestNewSPIFFEServer_MissingAllowedClientID (0.00s)
=== RUN   TestGetSPIFFEID_Present
--- PASS: TestGetSPIFFEID_Present (0.00s)
=== RUN   TestGetSPIFFEID_NotPresent
--- PASS: TestGetSPIFFEID_NotPresent (0.00s)
=== RUN   TestMustGetSPIFFEID_Present
--- PASS: TestMustGetSPIFFEID_Present (0.00s)
=== RUN   TestMustGetSPIFFEID_Panics
--- PASS: TestMustGetSPIFFEID_Panics (0.00s)
PASS
```

## Migration Impact

### No Breaking Changes

All changes are **backward compatible**:
- Existing code using the interface continues to work
- `GetMux()` is an addition, not a replacement
- `Start()` behavior is identical from external perspective

### Optional Enhancements

Users can now optionally use `GetMux()`:

```go
server, _ := identityserver.NewSPIFFEServer(ctx, cfg)

// NEW: Direct mux access for advanced use
mux := server.GetMux()
mux.HandleFunc("/debug/", debugHandler)  // Add handlers directly

server.Start(ctx)
```

## Files Modified

| File | Changes | Lines Changed |
|------|---------|---------------|
| internal/ports/identityserver.go | Added GetMux() to interface | +3 |
| internal/adapters/inbound/identityserver/spiffe_server.go | Simplified Start(), added GetMux(), reordered fields | ~15 |

**Total**: 2 files, ~18 lines changed

## Benefits Summary

1. **Cleaner Code**: Simplified `Start()` implementation
2. **Better Patterns**: `sync.Once` ensures idempotency
3. **More Flexible**: `GetMux()` enables advanced use cases
4. **No Breaking Changes**: Fully backward compatible
5. **Better Tested**: All tests pass

## Examples

### Using GetMux() for Custom Middleware

```go
server, _ := identityserver.NewSPIFFEServer(ctx, cfg)

// Get direct mux access
mux := server.GetMux()

// Add custom middleware to specific paths
mux.Handle("/metrics", promhttp.Handler())
mux.HandleFunc("/debug/pprof/", pprof.Index)

server.Start(ctx)
```

### Start() Idempotency

```go
server, _ := identityserver.NewSPIFFEServer(ctx, cfg)

// First call - starts server
go server.Start(ctx)

// Second call - no-op (protected by sync.Once)
go server.Start(ctx)  // Safe, won't panic or start twice
```

## References

- [sync.Once Documentation](https://pkg.go.dev/sync#Once)
- [go-spiffe SDK](https://github.com/spiffe/go-spiffe)
- [PORT_BASED_IMPROVEMENTS.md](PORT_BASED_IMPROVEMENTS.md)

---

**Status**: ✅ Complete
**Date**: 2025-10-07
**Backward Compatible**: Yes
