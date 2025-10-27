# docs/README.md

```markdown
# Documentation

This documentation uses the [Diátaxis framework](https://diataxis.fr/) for clarity and ease of navigation.

## Documentation Types

### [Tutorials](tutorials/) - **Learning-Oriented**

*Start here if you're new to the project.*

Step-by-step introductions that teach you how to use the system through hands-on examples.

- **[Quick Start](tutorials/QUICKSTART.md)** - Get up and running in 5 minutes
- **[Editor Setup](tutorials/EDITOR_SETUP.md)** - Configure your IDE for development
- **[Prerequisites](tutorials/examples/PREREQUISITES.md)** - Essential background before running examples
- **[Examples](tutorials/examples/)** - Hands-on mTLS server and client examples

**When to use**: You want to **learn** by doing and need guided practice.

---

### [How-To Guides](how-to-guides/) - **Task-Oriented**

*Come here when you have a specific goal to achieve.*

Practical solutions for specific tasks and problems you'll encounter in real-world usage.

**Deployment & Operations**:
- **[Production Workload API](how-to-guides/PRODUCTION_WORKLOAD_API.md)** - Deploy with kernel-level attestation
- **[Troubleshooting](how-to-guides/TROUBLESHOOTING.md)** - Debug common issues

**Development & Testing**:
- **[CodeQL Local Setup](how-to-guides/codeql-local-setup.md)** - Run security analysis locally
- **[Security Tools](how-to-guides/security-tools.md)** - Set up security scanning

**Workarounds & Fixes**:
- **[SPIRE Distroless Workaround](how-to-guides/SPIRE_DISTROLESS_WORKAROUND.md)** - Fix distroless image issues

**When to use**: You know **what** you want to do and need **how** to do it.

---

### [Reference](reference/) - **Information-Oriented**

*Look here when you need precise technical details.*

Authoritative specifications, APIs, contracts, and technical descriptions.

**Architecture Contracts**:
- **[Port Contracts](reference/PORT_CONTRACTS.md)** - Interface definitions and contracts
- **[Invariants](reference/INVARIANTS.md)** - System guarantees and assumptions
- **[Domain Model](reference/DOMAIN.md)** - Core domain types and rules

**Testing**:
- **[Test Architecture](reference/TEST_ARCHITECTURE.md)** - How tests are organized
- **[Testing Guide](reference/TESTING_GUIDE.md)** - Comprehensive testing documentation
- **[Integration Test Optimization](reference/INTEGRATION_TEST_OPTIMIZATION.md)** - Performance improvements
- **[End-to-End Tests](reference/END_TO_END_TESTS.md)** - Full system testing
- **[Property-Based Testing](reference/pbt.md)** - PBT patterns and practices

**Verification**:
- **[Verification](reference/VERIFICATION.md)** - System validation procedures

**When to use**: You need **accurate**, **complete** information about how something works.

---

### [Explanation](explanation/) - **Understanding-Oriented**

*Read these to understand the "why" behind the design.*

Background, rationale, and deep dives into design decisions and architectural choices.

**Architecture & Design**:
- **[Architecture](explanation/ARCHITECTURE.md)** - System architecture overview
- **[Architecture Review](explanation/ARCHITECTURE_REVIEW.md)** - Design decisions and trade-offs
- **[Design by Contract](explanation/DESIGN_BY_CONTRACT.md)** - Why we use contracts

**Evolution & Decisions**:
- **[SPIFFE ID Refactoring](explanation/SPIFFE_ID_REFACTORING.md)** - Why we refactored identity handling
- **[Unified Config Improvements](explanation/UNIFIED_CONFIG_IMPROVEMENTS.md)** - Why config was unified
- **[Iterations Summary](explanation/ITERATIONS_SUMMARY.md)** - Project evolution history

**Features & Patterns**:
- **[Debug Mode](explanation/DEBUG_MODE.md)** - Why and how debug mode works
- **[Refactoring Patterns](explanation/REFACTORING_PATTERNS.md)** - Common refactoring approaches

**Project Status**:
- **[Project Setup Status](explanation/PROJECT_SETUP_STATUS.md)** - Current state and roadmap

**When to use**: You want to **understand** the reasoning, history, or context behind decisions.

---

## Quick Navigation

### I'm a **new user**
→ Start with **[Tutorials](tutorials/)** to learn the basics

### I need to **solve a problem**
→ Check **[How-To Guides](how-to-guides/)** for practical solutions

### I need **technical details**
→ Look in **[Reference](reference/)** for specifications

### I want to **understand the design**
→ Read **[Explanation](explanation/)** for context and rationale

---

## Diátaxis Framework

This documentation structure follows the Diátaxis framework, which organizes documentation by **user needs**:

|                | **Practical** | **Theoretical** |
|----------------|---------------|-----------------|
| **Learning**   | Tutorials     | Explanation     |
| **Working**    | How-to guides | Reference       |

**Benefits**:
- ✅ Easy to find what you need based on your current goal
- ✅ Clear separation between learning, doing, and understanding
- ✅ Consistent organization across the entire project
- ✅ Reduces cognitive load when navigating documentation

Learn more about Diátaxis at [diataxis.fr](https://diataxis.fr/)

---

## External Resources

- **[Main README](../README.md)** - Project overview and API reference
- **[Examples](tutorials/examples/)** - Hands-on code examples
- **[Contributing](#)** - How to contribute (if you have a CONTRIBUTING.md)

---

## Documentation Metadata

Each document includes a header indicating its type:

```markdown
---
type: tutorial | how-to | reference | explanation
audience: beginner | intermediate | advanced
---
```

This helps you quickly identify if a document matches your needs.

---

## Contributing to Documentation

When adding new documentation:

1. **Identify the type**: Is it a tutorial, how-to guide, reference, or explanation?
2. **Place it correctly**: Put it in the appropriate folder
3. **Add metadata**: Include the document type header
4. **Update this index**: Add a link to the relevant section above
5. **Check links**: Ensure all cross-references work

### Decision Matrix: Where Does a New Doc Go?

**Is it teaching someone to use the system for the first time?**
→ `tutorials/`

**Is it solving a specific task or problem?**
→ `how-to-guides/`

**Is it documenting an API, contract, or specification?**
→ `reference/`

**Is it explaining why we made a design decision?**
→ `explanation/`

### Good Practices

- **Tutorials** should be complete, self-contained lessons
- **How-to guides** should focus on one specific task
- **Reference** docs should be comprehensive and precise
- **Explanations** should provide context, not instructions

---

## Still Can't Find What You Need?

- Check the **[main README](../README.md)** for an overview
- Browse **[examples/](tutorials/examples/)** for code samples
- Open an issue if documentation is missing or unclear
```

# internal/adapters/outbound/spire/identity_service_debug.go

```go
//go:build debug

package spire

import (
	"context"
	"time"

	"github.com/pocket/hexagon/spire/internal/debug"
)

// Ensure IdentityServiceSPIRE implements debug.Introspector in debug builds.
// This compile-time assertion verifies the interface is satisfied.
var _ debug.Introspector = (*IdentityServiceSPIRE)(nil)

// SnapshotData returns a sanitized view of the current SPIRE identity state.
//
// This method is only available in debug builds (via //go:build debug tag).
// It provides a safe view of identity information for debugging without
// exposing secrets like private keys or raw certificate data.
//
// WARNING: This endpoint should NEVER be exposed in production builds.
// The build tag ensures it's compiled out of production binaries.
//
// Implementation notes:
//   - Fetches current X.509 SVID from SPIRE Agent
//   - Calculates certificate expiration time (not the raw cert)
//   - Returns only public identity information (SPIFFE IDs, expiration times)
//   - MUST NOT include private keys, raw certificates, or sensitive data
//   - Errors are surfaced as synthetic AuthDecision entries with Decision: "ERROR"
//
// Concurrency: Safe for concurrent use (delegates to thread-safe Client).
func (s *IdentityServiceSPIRE) SnapshotData(ctx context.Context) debug.Snapshot {
	snapshot := debug.Snapshot{
		Mode:            debug.Active.Mode, // Configured via SPIRE_DEBUG_MODE env var
		Adapter:         "spire",           // Using real SPIRE (not inmemory)
		Certs:           []debug.CertView{},
		RecentDecisions: []debug.AuthDecision{},
	}

	// Attempt to fetch current identity document
	doc, err := s.client.FetchX509SVID(ctx)
	if err != nil {
		// Don't overload TrustDomain with error messages.
		// Instead, surface error as synthetic AuthDecision with Decision: "ERROR"
		// This keeps the schema stable and allows clients to parse errors properly.
		snapshot.RecentDecisions = append(snapshot.RecentDecisions, debug.AuthDecision{
			CallerSPIFFEID: "",
			Resource:       "spire.FetchX509SVID",
			Decision:       "ERROR",
			Reason:         err.Error(),
		})
		return snapshot
	}

	// Extract trust domain and certificate info if we got a valid document
	if doc != nil {
		cred := doc.IdentityCredential()
		if cred != nil {
			snapshot.TrustDomain = cred.TrustDomainString()

			// Calculate time until expiration (negative if already expired)
			expiresIn := time.Until(doc.ExpiresAt()).Seconds()

			snapshot.Certs = append(snapshot.Certs, debug.CertView{
				SpiffeID:         cred.SPIFFEID(),
				ExpiresInSeconds: int64(expiresIn),
				// TODO: Plumb real rotation status if available from SPIRE
				RotationPending: false, // SPIRE handles rotation transparently
			})
		}
	}

	return snapshot
}
```

# internal/app/application.go

```go
package app

import (
	"context"
	"fmt"
	"net/http"
	"time"

	// debug is imported in all builds because Application holds a *debug.Server.
	// In debug builds this is the real HTTP debug server; in non-debug builds
	// it's a stub type (see internal/debug/server_stub.go). The shared field +
	// method (`SetDebugServer`, `Close`) must compile in both cases so shutdown
	// behavior stays consistent across build tags.
	"github.com/pocket/hexagon/spire/internal/debug"

	"github.com/pocket/hexagon/spire/internal/dto"
	"github.com/pocket/hexagon/spire/internal/ports"
)

// Application wires application dependencies.
//
// Identity Operations:
//   - Use IdentityService for SPIRE-agnostic identity operations (preferred)
//   - Agent provides lower-level SPIRE operations (use only when necessary)
//
// The IdentityService abstracts SPIRE implementation details and returns
// ports.Identity, making application code independent of SPIRE-specific types.
type Application struct {
	cfg             *dto.Config
	agent           ports.Agent
	identityService ports.IdentityService
	debugServer     *debug.Server // nil in production builds or if debug server not started
}

// New constructs an Application and validates required deps.
func New(cfg *dto.Config, agent ports.Agent, identityService ports.IdentityService) (*Application, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if agent == nil {
		return nil, fmt.Errorf("agent is nil")
	}
	if identityService == nil {
		return nil, fmt.Errorf("identity service is nil")
	}
	return &Application{
		cfg:             cfg,
		agent:           agent,
		identityService: identityService,
	}, nil
}

// Close releases resources owned by the application (idempotent).
// Shutdown order:
//  1. Stop debug server (graceful, 5s timeout) so no new requests race with teardown.
//  2. Close agent.
// Safe to call multiple times.
func (a *Application) Close() error {
	if a == nil {
		return nil
	}

	var firstErr error

	// Stop debug server first (graceful shutdown with timeout)
	if a.debugServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.debugServer.Stop(ctx); err != nil && err != http.ErrServerClosed {
			// http.ErrServerClosed is expected on repeated calls - treat as non-fatal
			// We can't rely on debug.GetLogger() in !debug builds (no stub),
			// so capture the first error and surface it to the caller instead.
			if firstErr == nil {
				firstErr = fmt.Errorf("error stopping debug server: %w", err)
			}
		}
		a.debugServer = nil // Clear to ensure idempotence
	}

	// Then close agent
	if a.agent != nil {
		if err := a.agent.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		a.agent = nil // Clear to ensure idempotence
	}

	return firstErr
}

// SetDebugServer sets the debug server instance for graceful shutdown.
// This is called by BootstrapWithDebug in debug builds.
//
// In non-debug builds, debug.Server is a stub (see internal/debug/server_stub.go)
// and contains no active listener. Passing that stub here is harmless and will
// still satisfy Close(), which will see a nil or inert server and no-op.
func (a *Application) SetDebugServer(srv *debug.Server) {
	if a != nil {
		a.debugServer = srv
	}
}

// Accessors (add only what you need)
func (a *Application) Config() *dto.Config                   { return a.cfg }
func (a *Application) Agent() ports.Agent                    { return a.agent }
func (a *Application) IdentityService() ports.IdentityService { return a.identityService }
```

# internal/app/bootstrap_debug.go

```go
//go:build debug

package app

import (
	"context"
	"fmt"

	"github.com/pocket/hexagon/spire/internal/adapters/outbound/spire"
	"github.com/pocket/hexagon/spire/internal/debug"
	"github.com/pocket/hexagon/spire/internal/dto"
	"github.com/pocket/hexagon/spire/internal/ports"
	spireLib "github.com/spiffe/go-spiffe/v2/workloadapi"
)

// BootstrapWithDebug creates an Application with full debug instrumentation.
//
// Debug features:
//   - Starts local HTTP debug server (if SPIRE_DEBUG_SERVER=localhost:6060)
//   - Enables fault injection endpoints for testing failure modes
//   - Exposes /_debug/identity endpoint for certificate introspection
//
// Security:
//   - Debug server is loopback-only (127.0.0.1, ::1, localhost) - NEVER exposed to external networks
//   - Only available in debug builds (via //go:build debug tag)
//   - Production builds get a no-op stub (see bootstrap.go) that compiles out all debug code
//
// This function is only compiled in debug builds. Production builds use the stub
// in bootstrap.go which has no debug server initialization.
func BootstrapWithDebug(ctx context.Context, cfg *dto.Config) (*Application, error) {
	// Initialize debug mode from environment variables
	debug.Init()

	// Create SPIRE client
	client, err := spireLib.New(ctx, spireLib.WithAddr(cfg.SPIRE.SocketPath))
	if err != nil {
		return nil, fmt.Errorf("failed to create SPIRE client: %w", err)
	}

	// Wrap client in SPIRE agent adapter
	agent := spire.NewAgent(client)

	// Create identity service (implements both ports.IdentityService and debug.Introspector)
	var identityService ports.IdentityService
	identityService = spire.NewIdentityService(client)

	// Start debug server with introspection capabilities
	// The introspector provides /_debug/identity endpoint
	var introspector debug.Introspector
	if svc, ok := identityService.(debug.Introspector); ok {
		introspector = svc
	}
	debugServer := debug.Start(introspector)

	// Create application
	app, err := New(cfg, agent, identityService)
	if err != nil {
		// Clean up on error
		if debugServer != nil {
			_ = debugServer.Stop(ctx)
		}
		_ = agent.Close()
		return nil, err
	}

	// Wire debug server for graceful shutdown
	app.SetDebugServer(debugServer)

	debug.GetLogger().Debug("✅ Application bootstrapped with debug mode enabled")
	return app, nil
}
```

# internal/debug/config.go

```go
package debug

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds debug feature flags parsed from env vars.
// All debug features default to OFF in production.
type Config struct {
	// Enabled determines if debug mode is active (SPIRE_DEBUG=true)
	Enabled bool

	// Mode controls which debug features are enabled:
	//   - "debug": All features (server, fault injection, verbose logs)
	//   - "staging": Read-only endpoints only (no fault injection)
	//   - "production": Everything disabled (default)
	Mode string

	// Stress enables concurrent chaos testing (SPIRE_DEBUG_STRESS=true)
	Stress bool

	// SingleThreaded forces single-threaded execution for race detection (SPIRE_DEBUG_SINGLE_THREADED=true)
	SingleThreaded bool

	// LocalDebugServer determines if the HTTP debug server should start (SPIRE_DEBUG_SERVER set)
	LocalDebugServer bool

	// DebugServerAddr is the address to bind the debug server to (e.g., "localhost:6060")
	DebugServerAddr string
}

// Active holds the current debug configuration.
// Set by Init() during application startup.
var Active = Config{
	Enabled:          false,
	Mode:             "production",
	Stress:           false,
	SingleThreaded:   false,
	LocalDebugServer: false,
	DebugServerAddr:  "",
}

// Init loads debug configuration from environment variables.
// Call this once during application startup, before any debug features are used.
//
// Environment Variables:
//   - SPIRE_DEBUG: Set to "true" to enable debug mode
//   - SPIRE_DEBUG_MODE: Set to "debug", "staging", or "production" (default: inherits from SPIRE_DEBUG)
//   - SPIRE_DEBUG_STRESS: Set to "true" to enable stress testing
//   - SPIRE_DEBUG_SINGLE_THREADED: Set to "true" to force single-threaded execution
//   - SPIRE_DEBUG_SERVER: Set to "host:port" to enable local debug HTTP server (e.g., "localhost:6060")
//
// Example:
//
//	SPIRE_DEBUG=true SPIRE_DEBUG_SERVER=localhost:6060 ./myapp
func Init() {
	// Parse SPIRE_DEBUG flag
	Active.Enabled = parseBool(os.Getenv("SPIRE_DEBUG"))

	// Parse mode with fallback logic:
	// 1. If SPIRE_DEBUG_MODE is set, use it
	// 2. Otherwise, derive from SPIRE_DEBUG (true -> "debug", false -> "production")
	modeEnv := os.Getenv("SPIRE_DEBUG_MODE")
	if modeEnv != "" {
		Active.Mode = strings.ToLower(modeEnv)
	} else {
		if Active.Enabled {
			Active.Mode = "debug"
		} else {
			Active.Mode = "production"
		}
	}

	// Normalize mode to allowed values (prevents typos from creating undocumented modes)
	switch Active.Mode {
	case "debug", "staging", "production":
		// allowed
	default:
		// force a known-safe mode instead of trusting garbage
		Active.Mode = "debug"
	}

	// Parse optional feature flags
	Active.Stress = parseBool(os.Getenv("SPIRE_DEBUG_STRESS"))
	Active.SingleThreaded = parseBool(os.Getenv("SPIRE_DEBUG_SINGLE_THREADED"))

	// Parse debug server address
	serverAddr := os.Getenv("SPIRE_DEBUG_SERVER")
	if serverAddr != "" {
		Active.LocalDebugServer = true
		Active.DebugServerAddr = serverAddr
	}

	// Log the active configuration if we're in a mode that supports logging
	if Active.Mode == "debug" || Active.Mode == "staging" {
		logConfig()
	}
}

// parseBool converts a string to a boolean.
// Recognized true values: "1", "t", "true", "yes", "y" (case-insensitive)
// Everything else is false.
func parseBool(s string) bool {
	if s == "" {
		return false
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		// Try common case-insensitive variants
		lower := strings.ToLower(strings.TrimSpace(s))
		return lower == "yes" || lower == "y"
	}
	return b
}

// logConfig prints the active debug configuration.
// Only called if we're in debug or staging mode.
func logConfig() {
	fmt.Printf("🔧 Debug Configuration:\n")
	fmt.Printf("   Mode: %s\n", Active.Mode)
	fmt.Printf("   Enabled: %v\n", Active.Enabled)
	fmt.Printf("   Stress: %v\n", Active.Stress)
	fmt.Printf("   SingleThreaded: %v\n", Active.SingleThreaded)
	fmt.Printf("   LocalDebugServer: %v\n", Active.LocalDebugServer)
	if Active.LocalDebugServer {
		fmt.Printf("   DebugServerAddr: %s\n", Active.DebugServerAddr)
	}
}
```

# internal/debug/config_pbt_test.go

```go
package debug

import (
	"os"
	"testing"
	"testing/quick"
)

// TestParseBool_PropertyBased uses property-based testing to verify parseBool
// handles arbitrary string inputs without panicking.
func TestParseBool_PropertyBased(t *testing.T) {
	f := func(input string) bool {
		// parseBool should never panic
		_ = parseBool(input)
		return true
	}

	if err := quick.Check(f, nil); err != nil {
		t.Errorf("parseBool panicked on input: %v", err)
	}
}

// TestParseBool_TruthyValues verifies known truthy inputs.
func TestParseBool_TruthyValues(t *testing.T) {
	truthy := []string{"1", "t", "T", "true", "TRUE", "True", "yes", "YES", "Yes", "y", "Y"}
	for _, val := range truthy {
		if !parseBool(val) {
			t.Errorf("expected parseBool(%q) to be true", val)
		}
	}
}

// TestParseBool_FalsyValues verifies known falsy inputs.
func TestParseBool_FalsyValues(t *testing.T) {
	falsy := []string{"", "0", "f", "F", "false", "FALSE", "False", "no", "NO", "No", "n", "N", "garbage", "💩"}
	for _, val := range falsy {
		if parseBool(val) {
			t.Errorf("expected parseBool(%q) to be false", val)
		}
	}
}

// TestInit_DefaultValues verifies Init() sets safe defaults when no env vars are set.
func TestInit_DefaultValues(t *testing.T) {
	// Clear all debug-related env vars
	os.Unsetenv("SPIRE_DEBUG")
	os.Unsetenv("SPIRE_DEBUG_MODE")
	os.Unsetenv("SPIRE_DEBUG_STRESS")
	os.Unsetenv("SPIRE_DEBUG_SINGLE_THREADED")
	os.Unsetenv("SPIRE_DEBUG_SERVER")

	Init()

	if Active.Enabled {
		t.Errorf("expected Enabled=false, got %v", Active.Enabled)
	}
	if Active.Mode != "production" {
		t.Errorf("expected Mode=production, got %q", Active.Mode)
	}
	if Active.Stress {
		t.Errorf("expected Stress=false, got %v", Active.Stress)
	}
	if Active.SingleThreaded {
		t.Errorf("expected SingleThreaded=false, got %v", Active.SingleThreaded)
	}
	if Active.LocalDebugServer {
		t.Errorf("expected LocalDebugServer=false, got %v", Active.LocalDebugServer)
	}
	if Active.DebugServerAddr != "" {
		t.Errorf("expected DebugServerAddr empty, got %q", Active.DebugServerAddr)
	}
}

// TestInit_DebugModeEnabled verifies SPIRE_DEBUG=true enables debug mode.
func TestInit_DebugModeEnabled(t *testing.T) {
	os.Setenv("SPIRE_DEBUG", "true")
	defer os.Unsetenv("SPIRE_DEBUG")

	Init()

	if !Active.Enabled {
		t.Errorf("expected Enabled=true when SPIRE_DEBUG=true")
	}
	if Active.Mode != "debug" {
		t.Errorf("expected Mode=debug when SPIRE_DEBUG=true, got %q", Active.Mode)
	}
}

// TestInit_ExplicitMode verifies SPIRE_DEBUG_MODE overrides SPIRE_DEBUG.
func TestInit_ExplicitMode(t *testing.T) {
	os.Setenv("SPIRE_DEBUG", "true")
	os.Setenv("SPIRE_DEBUG_MODE", "staging")
	defer func() {
		os.Unsetenv("SPIRE_DEBUG")
		os.Unsetenv("SPIRE_DEBUG_MODE")
	}()

	Init()

	if Active.Mode != "staging" {
		t.Errorf("expected Mode=staging, got %q", Active.Mode)
	}
}

// TestInit_DebugServerEnabled verifies SPIRE_DEBUG_SERVER enables local debug server.
func TestInit_DebugServerEnabled(t *testing.T) {
	os.Setenv("SPIRE_DEBUG_SERVER", "localhost:6060")
	defer os.Unsetenv("SPIRE_DEBUG_SERVER")

	Init()

	if !Active.LocalDebugServer {
		t.Errorf("expected LocalDebugServer=true when SPIRE_DEBUG_SERVER is set")
	}
	if Active.DebugServerAddr != "localhost:6060" {
		t.Errorf("expected DebugServerAddr=localhost:6060, got %q", Active.DebugServerAddr)
	}
}

// TestInit_ModeNormalization verifies that invalid modes are normalized to "debug".
func TestInit_ModeNormalization(t *testing.T) {
	tests := []struct {
		name         string
		envMode      string
		expectedMode string
	}{
		{"empty", "", "production"},
		{"debug", "debug", "debug"},
		{"staging", "staging", "staging"},
		{"production", "production", "production"},
		{"garbage", "foobar", "debug"},
		{"uppercase", "DEBUG", "debug"},
		{"mixed case", "StAgInG", "staging"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save old env to restore after subtest
			oldDebug := os.Getenv("SPIRE_DEBUG")
			oldServer := os.Getenv("SPIRE_DEBUG_SERVER")
			oldMode := os.Getenv("SPIRE_DEBUG_MODE")

			// Restore after subtest
			defer func() {
				if oldDebug == "" {
					os.Unsetenv("SPIRE_DEBUG")
				} else {
					os.Setenv("SPIRE_DEBUG", oldDebug)
				}

				if oldServer == "" {
					os.Unsetenv("SPIRE_DEBUG_SERVER")
				} else {
					os.Setenv("SPIRE_DEBUG_SERVER", oldServer)
				}

				if oldMode == "" {
					os.Unsetenv("SPIRE_DEBUG_MODE")
				} else {
					os.Setenv("SPIRE_DEBUG_MODE", oldMode)
				}
			}()

			// Apply this subtest's env
			if tt.envMode == "" {
				os.Unsetenv("SPIRE_DEBUG_MODE")
			} else {
				os.Setenv("SPIRE_DEBUG_MODE", tt.envMode)
			}

			os.Unsetenv("SPIRE_DEBUG")
			os.Unsetenv("SPIRE_DEBUG_SERVER")

			Init()

			if Active.Mode != tt.expectedMode {
				t.Errorf("expected mode %q, got %q", tt.expectedMode, Active.Mode)
			}
		})
	}
}
```

# internal/debug/introspector.go

```go
package debug

import "context"

// Introspector is implemented by components that can provide debug snapshots.
//
// This interface is safe to compile in all builds (no build tags) because
// it's just an interface definition. The implementation is only provided
// in debug builds.
//
// Typical implementation: The identity service or application core
// implements this in a debug-only file to provide sanitized runtime state.
type Introspector interface {
	// SnapshotData returns a sanitized view of current identity state.
	//
	// MUST NOT include secrets (private keys, tokens, passwords).
	// Only return information safe for debugging:
	//   - Current SPIFFE IDs
	//   - Certificate expiration times
	//   - Recent authentication decisions
	//   - Adapter type (inmemory vs spire)
	SnapshotData(ctx context.Context) Snapshot
}
```

# internal/debug/server_debug.go

```go
//go:build debug

package debug

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	maxRequestBodyBytes = 10 * 1024 // 10KB max for fault injection requests
)

// Server is the debug HTTP server
type Server struct {
	addr         string
	mux          *http.ServeMux
	introspector Introspector
	httpServer   *http.Server
}

// FaultRequest represents a fault injection request
type FaultRequest struct {
	DropNextHandshake         *bool `json:"drop_next_handshake,omitempty"`
	CorruptNextSPIFFEID       *bool `json:"corrupt_next_spiffe_id,omitempty"`
	DelayNextIssueSeconds     *int  `json:"delay_next_issue_seconds,omitempty"`
	ForceTrustDomainMismatch  *bool `json:"force_trust_domain_mismatch,omitempty"`
	ForceExpiredCert          *bool `json:"force_expired_cert,omitempty"`
	RejectNextWorkloadLookup  *bool `json:"reject_next_workload_lookup,omitempty"`
}

// Start starts the debug HTTP server (debug build only).
// The server runs on localhost only and should never be exposed to external networks.
//
// The introspector parameter provides access to sanitized identity state.
// It can be nil, in which case the /_debug/identity endpoint will not be available.
//
// Returns the Server instance for graceful shutdown, or nil if the server was not started.
func Start(introspector Introspector) *Server {
	if !Active.LocalDebugServer {
		return nil
	}

	// Enforce loopback-only binding for security
	if Active.DebugServerAddr == "" {
		GetLogger().Debugf("REFUSING to start debug server: empty bind address")
		return nil
	}
	if !isLoopback(Active.DebugServerAddr) {
		// Clarify that non-loopback includes public IPs AND non-loopback hostnames.
		GetLogger().Debugf(
			"REFUSING to start debug server on non-loopback addr/host: %q (must be 127.0.0.0/8, ::1, or localhost)",
			Active.DebugServerAddr,
		)
		return nil
	}

	srv := &Server{
		addr:         Active.DebugServerAddr,
		mux:          http.NewServeMux(),
		introspector: introspector,
	}
	srv.registerHandlers()

	// Bind to the address before starting goroutine
	// This gives us the actual port when using :0 (ephemeral port)
	ln, err := net.Listen("tcp", srv.addr)
	if err != nil {
		GetLogger().Debugf("Failed to bind debug server: %v", err)
		return nil
	}

	// Create http.Server before starting goroutine so it can be shut down
	srv.httpServer = &http.Server{
		Addr:              ln.Addr().String(), // Use actual bound address
		Handler:           srv.mux,
		ReadHeaderTimeout: 2 * time.Second,  // Prevent Slowloris attacks
		IdleTimeout:       30 * time.Second, // Cap idle connection lifetime
		MaxHeaderBytes:    8 << 10,          // 8KB header limit
	}

	go func() {
		logger := GetLogger()
		// Safe to log: loopback-only address already validated by isLoopback().
		logger.Debugf("⚠️  DEBUG SERVER RUNNING ON %s", ln.Addr().String())
		logger.Debug("⚠️  WARNING: Debug mode is enabled. DO NOT USE IN PRODUCTION!")

		if err := srv.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Debugf("Debug server error: %v", err)
		}
	}()

	return srv
}

func (s *Server) registerHandlers() {
	s.mux.HandleFunc("/_debug/", s.handleIndex)
	s.mux.HandleFunc("/_debug/state", s.handleState)
	s.mux.HandleFunc("/_debug/faults", s.handleFaults)
	s.mux.HandleFunc("/_debug/faults/reset", s.handleFaultsReset)
	s.mux.HandleFunc("/_debug/config", s.handleConfig)
	s.mux.HandleFunc("/_debug/identity", s.handleIdentity)
}

// handleIndex serves the debug interface index page.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/_debug/" {
		http.NotFound(w, r)
		return
	}

	const html = `<!DOCTYPE html>
<html>
<head><title>SPIRE Debug</title></head>
<body>
<h1>SPIRE Identity Library - Debug Interface</h1>
<p><strong>⚠️ WARNING:</strong> This is a debug interface. Never use in production.</p>
<h2>Available Endpoints:</h2>
<ul>
<li><a href="/_debug/state">/_debug/state</a> - View current state</li>
<li><a href="/_debug/identity">/_debug/identity</a> - View identity snapshot (certs, auth decisions)</li>
<li><a href="/_debug/faults">/_debug/faults</a> - View/modify fault injection (GET/POST)</li>
<li><a href="/_debug/faults/reset">/_debug/faults/reset</a> - Reset all faults (POST)</li>
<li><a href="/_debug/config">/_debug/config</a> - View debug configuration</li>
</ul>
</body>
</html>`
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
}

// handleState exposes high-level debug runtime toggles and current fault state.
// This endpoint intentionally does NOT include secrets or identity material.
// Safe to serve in both "debug" and "staging" modes. Still loopback-only.
func (s *Server) handleState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	state := map[string]any{
		"debug_enabled": Active.Enabled,
		"mode":          Active.Mode, // "debug", "staging", or "production"
		"stress_mode":   Active.Stress,
		"single_thread": Active.SingleThreaded,
		"faults":        Faults.Snapshot(),
	}

	// NOTE: All debug JSON MUST be written via writeJSON / writeJSONStatus.
	// These helpers set Cache-Control: no-store and Content-Type correctly.
	// Do not inline json.NewEncoder(...).Encode(...) here.
	writeJSON(w, state)
}

// handleFaults handles GET and POST requests for fault injection.
// POST is only allowed in "debug" mode, not "staging" or "production".
func (s *Server) handleFaults(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.getFaults(w, r)
	case http.MethodPost:
		// Only allow mutation if we're explicitly in "debug" mode, not "staging".
		if Active.Mode != "debug" {
			http.Error(w, "Fault injection disabled in this mode", http.StatusForbidden)
			return
		}
		s.setFaults(w, r)
	default:
		methodNotAllowed(w)
	}
}

// getFaults returns the current fault configuration.
func (s *Server) getFaults(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, Faults.Snapshot())
}

// setFaults applies fault injection configuration from JSON request.
func (s *Server) setFaults(w http.ResponseWriter, r *http.Request) {
	logger := GetLogger()

	// Limit request body size to prevent DoS
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req FaultRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		logger.Debugf("Failed to decode fault request: %v", err)
		http.Error(w, fmt.Sprintf("Invalid JSON: %v", err), http.StatusBadRequest)
		return
	}

	// Apply faults using type-safe struct fields
	if req.DropNextHandshake != nil {
		Faults.SetDropNextHandshake(*req.DropNextHandshake)
		logger.Debugf("Fault set: drop_next_handshake=%v", *req.DropNextHandshake)
	}

	if req.CorruptNextSPIFFEID != nil {
		Faults.SetCorruptNextSPIFFEID(*req.CorruptNextSPIFFEID)
		logger.Debugf("Fault set: corrupt_next_spiffe_id=%v", *req.CorruptNextSPIFFEID)
	}

	if req.DelayNextIssueSeconds != nil {
		if err := Faults.SetDelayNextIssue(*req.DelayNextIssueSeconds); err != nil {
			logger.Debugf("Invalid delay value: %v", err)
			http.Error(w, fmt.Sprintf("Invalid delay: %v", err), http.StatusBadRequest)
			return
		}
		logger.Debugf("Fault set: delay_next_issue_seconds=%d", *req.DelayNextIssueSeconds)
	}

	if req.ForceTrustDomainMismatch != nil {
		Faults.SetForceTrustDomainMismatch(*req.ForceTrustDomainMismatch)
		logger.Debugf("Fault set: force_trust_domain_mismatch=%v", *req.ForceTrustDomainMismatch)
	}

	if req.ForceExpiredCert != nil {
		Faults.SetForceExpiredCert(*req.ForceExpiredCert)
		logger.Debugf("Fault set: force_expired_cert=%v", *req.ForceExpiredCert)
	}

	if req.RejectNextWorkloadLookup != nil {
		Faults.SetRejectNextWorkloadLookup(*req.RejectNextWorkloadLookup)
		logger.Debugf("Fault set: reject_next_workload_lookup=%v", *req.RejectNextWorkloadLookup)
	}

	// Return current state
	s.getFaults(w, r)
}

// handleFaultsReset resets all fault injections.
// Only allowed in "debug" mode, not "staging" or "production".
func (s *Server) handleFaultsReset(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
		return
	}

	// Only allow mutation if we're explicitly in "debug" mode, not "staging".
	if Active.Mode != "debug" {
		http.Error(w, "Fault injection disabled in this mode", http.StatusForbidden)
		return
	}

	Faults.Reset()
	GetLogger().Debug("All faults reset")

	writeJSON(w, map[string]string{"status": "reset"})
}

// handleConfig returns the current debug configuration.
func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	config := map[string]any{
		"enabled":            Active.Enabled,
		"mode":               Active.Mode,
		"stress":             Active.Stress,
		"single_threaded":    Active.SingleThreaded,
		"local_debug_server": Active.LocalDebugServer,
		"debug_server_addr":  Active.DebugServerAddr,
	}

	writeJSON(w, config)
}

// handleIdentity returns a snapshot of the current identity state.
// This endpoint is only available if an introspector was provided to Start().
// Returns 503 Service Unavailable if identity state has errors.
func (s *Server) handleIdentity(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		methodNotAllowed(w)
		return
	}

	if s.introspector == nil {
		http.Error(w, "Identity introspection not available (no introspector provided)", http.StatusNotImplemented)
		return
	}

	snapshot := s.introspector.SnapshotData(r.Context())

	// Return 503 if any errors in identity state
	status := http.StatusOK
	for _, d := range snapshot.RecentDecisions {
		if d.Decision == "ERROR" {
			status = http.StatusServiceUnavailable
			break
		}
	}

	// NOTE: All debug JSON MUST be written via writeJSON / writeJSONStatus.
	// These helpers enforce the no-store header. See writeJSONStatus docs/tests.
	writeJSONStatus(w, status, snapshot)
}

// writeJSONStatus writes a JSON response with the given status code.
// Security contract:
//   • Sets "Cache-Control: no-store" on ALL debug JSON responses to prevent
//     intermediaries, CLIs, or proxies from caching or logging operational state
//     (trust domains, SPIFFE IDs, fault toggles, etc.).
//   • Sets Content-Type to application/json; charset=utf-8.
//   • All debug endpoints MUST use this helper (or writeJSON) to avoid drift.
// Any change to these headers MUST update TestServer_handleIdentity_NoStoreHeader.
func writeJSONStatus(w http.ResponseWriter, status int, v any) {
	// Prevent caching of potentially sensitive debug state
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// writeJSON writes a JSON response with 200 OK status and proper content type.
func writeJSON(w http.ResponseWriter, v any) {
	writeJSONStatus(w, http.StatusOK, v)
}

// methodNotAllowed writes a 405 Method Not Allowed response.
// This intentionally does NOT use writeJSONStatus because it returns no sensitive
// runtime state (just a static error message). All endpoints that return operational
// data (state, identity, faults, etc.) MUST use writeJSON/writeJSONStatus.
func methodNotAllowed(w http.ResponseWriter) {
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

// Stop shuts down the debug server gracefully.
// Returns nil if the server was not started or is already stopped.
// The context controls the shutdown timeout.
// This method is idempotent - safe to call multiple times.
func (s *Server) Stop(ctx context.Context) error {
	if s == nil || s.httpServer == nil {
		return nil
	}

	GetLogger().Debug("Shutting down debug server")
	err := s.httpServer.Shutdown(ctx)

	// Make subsequent Stop() calls a no-op
	s.httpServer = nil

	return err
}

// isLoopback returns true if addr is a loopback-only listen address.
// Allowed forms:
//   - "127.x.y.z:port" (any 127.0.0.0/8 IPv4)
//   - "[::1]:port"     (IPv6 loopback)
//   - "localhost:port" (literal hostname only)
//
// Security contract:
//   • addr MUST include a port. Bare hosts like "127.0.0.1" are rejected
//     (SplitHostPort fails) and MUST remain rejected. This prevents someone
//     from "helpfully" accepting broader forms like "0.0.0.0" without port.
//   • Arbitrary hostnames are NOT allowed. Only literal "localhost" is
//     allowed. "api.example.com:6060" MUST be rejected even if it currently
//     resolves to 127.0.0.1 at runtime.
//   • Returning true here is what allows Start() to proceed and later log the
//     bound address. That log line is considered safe *only because* of this
//     check. Weakening this function without updating that log is a data leak.
//
// Any change to this logic MUST update tests that cover Start()/isLoopback.
func isLoopback(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil || host == "" {
		return false
	}

	// Allow literal "localhost" to reduce footguns in dev configs.
	if host == "localhost" {
		return true
	}

	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}
```

# internal/debug/server_debug_test.go

```go
//go:build debug

package debug

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestIsLoopback_Allowed verifies that loopback addresses are accepted.
func TestIsLoopback_Allowed(t *testing.T) {
	allowed := []string{
		"127.0.0.1:6060",
		"127.0.0.1:0",
		"127.1.2.3:8080",
		"[::1]:6060",
		"localhost:6060",
		"localhost:0",
	}

	for _, addr := range allowed {
		if !isLoopback(addr) {
			t.Errorf("expected %q to be loopback", addr)
		}
	}
}

// TestIsLoopback_Rejected verifies that non-loopback addresses are rejected.
func TestIsLoopback_Rejected(t *testing.T) {
	rejected := []string{
		"0.0.0.0:6060",
		"192.168.1.1:6060",
		"10.0.0.1:6060",
		"[::]:6060",
		"example.com:6060",
		"api.example.com:6060",
		"127.0.0.1",         // missing port
		"localhost",         // missing port
		":6060",             // missing host
		"",                  // empty
		"not-a-valid-addr:", // malformed
	}

	for _, addr := range rejected {
		if isLoopback(addr) {
			t.Errorf("expected %q to be rejected as non-loopback", addr)
		}
	}
}

// TestStart_RefusesNonLoopback verifies that Start() refuses non-loopback addresses.
func TestStart_RefusesNonLoopback(t *testing.T) {
	// Save and restore Active config
	oldActive := Active
	defer func() { Active = oldActive }()

	Active = Config{
		LocalDebugServer: true,
		DebugServerAddr:  "0.0.0.0:6060", // Non-loopback
	}

	srv := Start(nil)
	if srv != nil {
		t.Fatal("expected Start() to refuse non-loopback address")
	}
}

// TestStart_AcceptsLoopback verifies that Start() accepts loopback addresses.
func TestStart_AcceptsLoopback(t *testing.T) {
	// Save and restore Active config
	oldActive := Active
	defer func() { Active = oldActive }()

	Active = Config{
		LocalDebugServer: true,
		DebugServerAddr:  "127.0.0.1:0", // Loopback with ephemeral port
	}

	srv := Start(nil)
	if srv == nil {
		t.Fatal("expected Start() to accept loopback address")
	}
	defer srv.Stop(context.Background())
}

// TestStart_ReturnsNilWhenDisabled verifies that Start() returns nil when LocalDebugServer is false.
func TestStart_ReturnsNilWhenDisabled(t *testing.T) {
	// Save and restore Active config
	oldActive := Active
	defer func() { Active = oldActive }()

	Active = Config{
		LocalDebugServer: false,
		DebugServerAddr:  "127.0.0.1:6060",
	}

	srv := Start(nil)
	if srv != nil {
		t.Fatal("expected Start() to return nil when LocalDebugServer=false")
	}
}

// TestServer_handleState verifies /_debug/state returns expected JSON structure.
func TestServer_handleState(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/state", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"debug_enabled"`) {
		t.Errorf("expected 'debug_enabled' in response, got: %s", body)
	}
	if !strings.Contains(body, `"mode"`) {
		t.Errorf("expected 'mode' in response, got: %s", body)
	}
}

// TestServer_handleFaults_GET verifies /_debug/faults GET returns fault state.
func TestServer_handleFaults_GET(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/faults", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"drop_next_handshake"`) {
		t.Errorf("expected 'drop_next_handshake' in response, got: %s", body)
	}
}

// TestServer_handleFaults_POST_DebugMode verifies fault injection is allowed in debug mode.
func TestServer_handleFaults_POST_DebugMode(t *testing.T) {
	// Save and restore Active config
	oldActive := Active
	defer func() { Active = oldActive }()

	Active = Config{Mode: "debug"}

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	payload := strings.NewReader(`{"drop_next_handshake": true}`)
	req := httptest.NewRequest(http.MethodPost, "/_debug/faults", payload)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 in debug mode, got %d", w.Code)
	}
}

// TestServer_handleFaults_POST_StagingMode verifies fault injection is forbidden in staging mode.
func TestServer_handleFaults_POST_StagingMode(t *testing.T) {
	// Save and restore Active config
	oldActive := Active
	defer func() { Active = oldActive }()

	Active = Config{Mode: "staging"}

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	payload := strings.NewReader(`{"drop_next_handshake": true}`)
	req := httptest.NewRequest(http.MethodPost, "/_debug/faults", payload)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 in staging mode, got %d", w.Code)
	}
}

// TestServer_handleFaultsReset verifies /_debug/faults/reset works in debug mode.
func TestServer_handleFaultsReset(t *testing.T) {
	// Save and restore Active config
	oldActive := Active
	defer func() { Active = oldActive }()

	Active = Config{Mode: "debug"}

	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodPost, "/_debug/faults/reset", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"status"`) {
		t.Errorf("expected 'status' in response, got: %s", body)
	}
}

// TestServer_handleConfig verifies /_debug/config returns expected JSON structure.
func TestServer_handleConfig(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/config", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"enabled"`) {
		t.Errorf("expected 'enabled' in response, got: %s", body)
	}
	if !strings.Contains(body, `"mode"`) {
		t.Errorf("expected 'mode' in response, got: %s", body)
	}
}

// TestServer_handleIdentity_NoIntrospector verifies 501 when no introspector is provided.
func TestServer_handleIdentity_NoIntrospector(t *testing.T) {
	srv := &Server{mux: http.NewServeMux(), introspector: nil}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/identity", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotImplemented {
		t.Fatalf("expected 501, got %d", w.Code)
	}
}

// TestServer_Stop_Idempotent verifies that Stop() can be called multiple times safely.
func TestServer_Stop_Idempotent(t *testing.T) {
	// Save and restore Active config
	oldActive := Active
	defer func() { Active = oldActive }()

	Active = Config{
		LocalDebugServer: true,
		DebugServerAddr:  "127.0.0.1:0",
	}

	srv := Start(nil)
	if srv == nil {
		t.Fatal("expected Start() to succeed")
	}

	ctx := context.Background()

	// First stop
	if err := srv.Stop(ctx); err != nil && err != http.ErrServerClosed {
		t.Fatalf("first Stop() failed: %v", err)
	}

	// Second stop (should be no-op)
	if err := srv.Stop(ctx); err != nil {
		t.Fatalf("second Stop() failed: %v", err)
	}
}

// mockIntrospector is a test implementation of Introspector.
type mockIntrospector struct{}

func (m *mockIntrospector) SnapshotData(ctx context.Context) Snapshot {
	return Snapshot{
		Mode:        "debug",
		TrustDomain: "spiffe://example.org",
		Adapter:     "inmemory",
		Certs: []CertView{
			{
				SpiffeID:         "spiffe://example.org/test",
				ExpiresInSeconds: 3600,
				RotationPending:  false,
			},
		},
		RecentDecisions: []AuthDecision{
			{
				CallerSPIFFEID: "spiffe://example.org/client",
				Resource:       "api/v1/health",
				Decision:       "ALLOW",
				Reason:         "valid certificate",
			},
		},
	}
}

// TestServer_handleIdentity_WithIntrospector verifies /_debug/identity returns snapshot.
func TestServer_handleIdentity_WithIntrospector(t *testing.T) {
	srv := &Server{
		mux:          http.NewServeMux(),
		introspector: &mockIntrospector{},
	}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/identity", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"mode"`) {
		t.Errorf("expected 'mode' in response, got: %s", body)
	}
	if !strings.Contains(body, `"trustDomain"`) {
		t.Errorf("expected 'trustDomain' in response, got: %s", body)
	}
	if !strings.Contains(body, `"certs"`) {
		t.Errorf("expected 'certs' in response, got: %s", body)
	}
	if !strings.Contains(body, `"recentDecisions"`) {
		t.Errorf("expected 'recentDecisions' in response, got: %s", body)
	}
}

// TestServer_handleIdentity_NoStoreHeader verifies Cache-Control: no-store is set.
func TestServer_handleIdentity_NoStoreHeader(t *testing.T) {
	srv := &Server{
		mux:          http.NewServeMux(),
		introspector: &mockIntrospector{},
	}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/identity", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	cacheControl := w.Header().Get("Cache-Control")
	if cacheControl != "no-store" {
		t.Errorf("expected Cache-Control: no-store, got %q", cacheControl)
	}
}

// mockErrorIntrospector returns a snapshot with an ERROR decision.
type mockErrorIntrospector struct{}

func (m *mockErrorIntrospector) SnapshotData(ctx context.Context) Snapshot {
	return Snapshot{
		Mode:        "debug",
		TrustDomain: "",
		Adapter:     "spire",
		Certs:       []CertView{},
		RecentDecisions: []AuthDecision{
			{
				CallerSPIFFEID: "",
				Resource:       "spire.FetchX509SVID",
				Decision:       "ERROR",
				Reason:         "connection refused",
			},
		},
	}
}

// TestServer_handleIdentity_Error503 verifies 503 when snapshot contains errors.
func TestServer_handleIdentity_Error503(t *testing.T) {
	srv := &Server{
		mux:          http.NewServeMux(),
		introspector: &mockErrorIntrospector{},
	}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/identity", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 when snapshot has errors, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, `"ERROR"`) {
		t.Errorf("expected ERROR decision in response, got: %s", body)
	}
}

// TestMethodNotAllowed_DoesNotSetNoStoreHeader verifies that 405 responses
// do NOT set Cache-Control: no-store, because they return no runtime data.
func TestMethodNotAllowed_DoesNotSetNoStoreHeader(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodPost, "/_debug/state", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}

	// 405 is allowed to omit Cache-Control: no-store because it returns no runtime data.
	if got := w.Header().Get("Cache-Control"); got != "" {
		t.Errorf("did not expect Cache-Control header on 405, got %q", got)
	}

	// Sanity check: response is plain text, not JSON with operational state.
	body := w.Body.String()
	if body == "" {
		t.Fatalf("expected non-empty response body")
	}
	if body[0] == '{' {
		// Truncate for readability without introducing a package-level helper.
		preview := body
		if len(preview) > 20 {
			preview = preview[:20]
		}
		t.Errorf("405 response must not be JSON; got body starting with %q", preview)
	}
}

// TestServer_handleIndex verifies the index page is served correctly.
func TestServer_handleIndex(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "SPIRE Identity Library") {
		t.Errorf("expected index page to contain 'SPIRE Identity Library', got: %s", body)
	}
}

// TestServer_handleIndex_404 verifies 404 for non-root paths under /_debug/.
func TestServer_handleIndex_404(t *testing.T) {
	srv := &Server{mux: http.NewServeMux()}
	srv.registerHandlers()

	req := httptest.NewRequest(http.MethodGet, "/_debug/nonexistent", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for non-existent path, got %d", w.Code)
	}
}
```

# internal/debug/server_stub.go

```go
//go:build !debug

package debug

import "context"

// Server is a stub implementation of the debug server used in non-debug builds.
// In production builds, no debug HTTP server is started and this type carries no state.
type Server struct{}

// Start is a no-op stub in production builds.
func Start(introspector Introspector) *Server {
	return nil
}

// Stop is a no-op stub in production builds.
func (s *Server) Stop(ctx context.Context) error {
	return nil
}
```

# internal/debug/snapshot_types.go

```go
package debug

// Snapshot is what we expose over /_debug/identity.
//
// ABSOLUTE RULE: This struct (and any nested structs like CertView/AuthDecision)
// MUST NEVER contain:
//   - private keys
//   - raw cert material / full PEM / JWTs / bearer tokens
//   - socket paths or network endpoints that are not already public-facing
//
// This file is intentionally built in ALL builds (no //go:build tag) so other
// packages can reference these types. The /_debug/identity endpoint that returns
// this data only exists in debug builds, but treating this struct as "safe for
// prod" is a design goal. Adding sensitive material here is a security bug.
type Snapshot struct {
	Mode            string         `json:"mode"`            // "debug", "staging", or "production"
	TrustDomain     string         `json:"trustDomain"`     // e.g., "spiffe://example.org"
	Adapter         string         `json:"adapter"`         // "inmemory" or "spire"
	Certs           []CertView     `json:"certs"`           // Current certificates
	RecentDecisions []AuthDecision `json:"recentDecisions"` // Recent auth decisions
}

// CertView provides a safe view of certificate information.
// Excludes private keys and raw certificate data.
type CertView struct {
	SpiffeID         string `json:"spiffeID"`         // e.g., "spiffe://example.org/server"
	ExpiresInSeconds int64  `json:"expiresInSeconds"` // Time until expiration (negative if expired)
	RotationPending  bool   `json:"rotationPending"`  // True if rotation is scheduled/in progress
}

// AuthDecision represents a single authentication decision.
// Used for debugging authorization logic.
type AuthDecision struct {
	CallerSPIFFEID string `json:"callerSPIFFEID"` // Who tried to authenticate
	Resource       string `json:"resource"`       // What resource was accessed
	Decision       string `json:"decision"`       // "ALLOW" or "DENY"
	Reason         string `json:"reason"`         // Human-readable reason
}
```
