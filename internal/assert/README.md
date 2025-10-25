# Design by Contract in Go

This package implements **Design by Contract** for the SPIRE library using build tags.

## Overview

Design by Contract uses three types of assertions:

1. **Preconditions** - What the caller must guarantee
2. **Postconditions** - What the function guarantees on success
3. **Invariants** - What must always be true for the object/system

## How It Works

### Production Builds (default)
```bash
go build ./cmd
```
- All assertions are **no-ops**
- Zero runtime overhead
- No panics from assertion failures

### Debug Builds
```bash
go build -tags=debug ./cmd
```
- All assertions are **active**
- Violations cause immediate panics
- Helps catch bugs during development

## Usage

### Preconditions

Validate **caller input** at function entry. Always return `error` for invalid input.

```go
func SetTimeout(seconds int) error {
    // Precondition: validate caller input
    if seconds < 0 {
        return fmt.Errorf("timeout must be non-negative, got %d", seconds)
    }

    // ... rest of function
}
```

**Don't use assertions for preconditions** - use explicit error returns so callers get clear feedback.

### Postconditions

Verify that **your function** produced the correct result.

```go
import "github.com/pocket/hexagon/spire/internal/assert"

func NewUser(email string) (*User, error) {
    // Precondition: validate input
    if !isValidEmail(email) {
        return nil, fmt.Errorf("invalid email: %s", email)
    }

    user := &User{
        ID:    generateID(),
        Email: normalizeEmail(email),
    }

    // Postcondition: verify our code is correct
    assert.Postcondition(user.ID != "", "user ID must not be empty after construction")
    assert.Postcondition(user.Email != "", "user email must not be empty")

    return user, nil
}
```

**In debug builds:** Panics if postcondition fails (indicates bug in our code)
**In production builds:** No-op (zero overhead)

### Invariants

Verify that **system state** remains valid.

```go
func (f *FaultProfile) Snapshot() map[string]any {
    f.mu.RLock()
    defer f.mu.RUnlock()

    snapshot := map[string]any{
        "drop_next_handshake": f.DropNextHandshake,
        "corrupt_next_spiffe_id": f.CorruptNextSPIFFEID,
        // ... other fields
    }

    // Invariant: snapshot must always contain all fields
    assert.Invariant(len(snapshot) == 6, "snapshot must contain all 6 fault fields")

    return snapshot
}
```

**In debug builds:** Panics if invariant is violated
**In production builds:** No-op

## When to Use Each

| Type | When to Use | Error Handling |
|------|-------------|----------------|
| **Precondition** | Validate caller input | Return `error` (always active) |
| **Postcondition** | Verify function output | `assert.Postcondition()` (debug only) |
| **Invariant** | Verify system state | `assert.Invariant()` (debug only) |

## Examples from SPIRE Library

### Fault Injection (internal/debug/faults.go)

```go
// Precondition: seconds >= 0
// Postcondition: DelayNextIssueSeconds is set to the given value
func (f *FaultProfile) SetDelayNextIssue(seconds int) error {
    // Precondition: validate caller input
    if seconds < 0 {
        return fmt.Errorf("delay must be non-negative, got %d", seconds)
    }

    f.mu.Lock()
    defer f.mu.Unlock()
    f.DelayNextIssueSeconds = seconds

    // Postcondition: verify we actually set it correctly
    assert.Postcondition(f.DelayNextIssueSeconds == seconds,
        "DelayNextIssueSeconds must equal the value just set")

    return nil
}
```

### Snapshot Invariant

```go
// Postcondition: returned map contains all 6 fault keys
// Invariant: snapshot is always consistent (no partial updates visible)
func (f *FaultProfile) Snapshot() map[string]any {
    f.mu.RLock()
    defer f.mu.RUnlock()

    snapshot := map[string]any{
        "drop_next_handshake": f.DropNextHandshake,
        // ... all 6 fields
    }

    // Postcondition: verify snapshot completeness
    assert.Postcondition(len(snapshot) == 6,
        "snapshot must contain all 6 fault fields")

    return snapshot
}
```

## Testing Contracts

Write tests that verify postconditions:

```go
func TestSetDelayNextIssue_Postconditions(t *testing.T) {
    f := &FaultProfile{}

    err := f.SetDelayNextIssue(10)
    if err != nil {
        t.Fatal(err)
    }

    // Verify postcondition: value was actually set
    delay := f.GetAndClearDelay()
    if delay != 10 {
        t.Errorf("Expected delay=10, got %d", delay)
    }
}
```

## Benefits

1. **Debug builds catch bugs early** - Violations panic immediately during development
2. **Production builds have zero overhead** - Assertions are completely stripped
3. **Self-documenting code** - Contracts make expectations explicit
4. **Type safety** - Assertions prevent illegal states at compile time (via unexported fields)
5. **Domain alignment** - Contracts encode business rules in code

## Best Practices

### Do ✅

- Use preconditions for **input validation** (return `error`)
- Use postconditions to verify **your code is correct**
- Use invariants to protect **critical system state**
- Document contracts in godoc comments
- Write tests that verify postconditions

### Don't ❌

- Don't use assertions for user/external errors (use `error` returns)
- Don't rely on assertions in production (they're stripped!)
- Don't panic for bad caller input (return `error` instead)
- Don't skip precondition checks (always validate input)

## Architecture Integration

This follows hexagonal architecture principles:

- **Domain Layer**: Uses invariants to protect business rules
- **Port Layer**: Uses preconditions to validate inputs
- **Adapter Layer**: Converts external errors to domain errors

## See Also

- Eiffel language (origin of Design by Contract)
- "Object-Oriented Software Construction" by Bertrand Meyer
- Go build tags documentation
