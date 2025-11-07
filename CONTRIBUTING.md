# Contributing to e5s

Thank you for your interest in contributing to e5s! This document provides guidelines for contributing to the project.

## Code of Conduct

This project follows the [Contributor Covenant Code of Conduct](https://www.contributor-covenant.org/version/2/1/code_of_conduct/). By participating, you are expected to uphold this code.

## How to Contribute

### Reporting Bugs

Before creating bug reports, please check the existing issues to avoid duplicates. When creating a bug report, include:

- A clear and descriptive title
- Detailed steps to reproduce the problem
- Expected behavior vs actual behavior
- Your environment (Go version, OS, SPIRE version)
- Any relevant logs or error messages

Use the bug report template when creating an issue.

### Suggesting Enhancements

Enhancement suggestions are tracked as GitHub issues. When creating an enhancement suggestion, include:

- A clear and descriptive title
- A detailed description of the proposed functionality
- Explain why this enhancement would be useful
- List any alternative solutions you've considered

Use the feature request template when creating an issue.

### Pull Requests

1. **Fork the repository** and create your branch from `main`
2. **Follow the coding style** - Run `golangci-lint run` before submitting
3. **Write tests** - Ensure your changes are covered by tests
4. **Update documentation** - Update README.md or docs/ if needed
5. **Run tests** - Ensure `go test -race ./...` passes
6. **Run security checks** - Ensure `gosec ./...` and `govulncheck ./...` pass
7. **Sign your commits** - Use `git commit -s` to add a Signed-off-by line
8. **Write a clear commit message** - Follow conventional commit format

#### Commit Message Format

We follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
type(scope): brief description

Detailed explanation of the change.

Fixes #123
```

Types:
- `feat:` - New feature
- `fix:` - Bug fix
- `docs:` - Documentation changes
- `test:` - Adding or updating tests
- `refactor:` - Code refactoring
- `perf:` - Performance improvements
- `chore:` - Maintenance tasks
- `ci:` - CI/CD changes
- `security:` - Security fixes

#### Pull Request Process

1. Update the README.md or documentation with details of changes if applicable
2. Update tests to cover your changes
3. Ensure all CI checks pass (linting, tests, security scans)
4. Your PR will be reviewed by maintainers
5. Address any feedback from reviewers
6. Once approved, your PR will be merged

## Development Setup

### Prerequisites

**Core Development:**
- Go 1.25.3 or later
- golangci-lint
- gosec
- govulncheck

**Integration Testing (optional):**

See [docs/INTEGRATION_TESTING.md](docs/INTEGRATION_TESTING.md) for detailed setup instructions and version requirements.

### Local Development

1. Clone your fork:
   ```bash
   git clone https://github.com/YOUR_USERNAME/e5s.git
   cd e5s
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Run tests:
   ```bash
   go test -race -v ./...
   ```

4. Run linters:
   ```bash
   golangci-lint run
   gosec ./...
   ```

5. Run security checks:
   ```bash
   govulncheck ./...
   ```

### Running Integration Tests

Integration tests require a running SPIRE deployment. See the [integration tests documentation](docs/integration-tests.md) for setup instructions.

## Testing

- Write unit tests for new functionality
- Ensure tests are deterministic and don't rely on timing
- Use table-driven tests where appropriate
- Mock external dependencies (SPIRE Workload API)
- Test error cases and edge conditions

Example test structure:
```go
func TestFeature(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    string
        wantErr bool
    }{
        // Test cases
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got, err := Feature(tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Feature() error = %v, wantErr %v", err, tt.wantErr)
                return
            }
            if got != tt.want {
                t.Errorf("Feature() = %v, want %v", got, tt.want)
            }
        })
    }
}
```

## Security

If you discover a security vulnerability, **do not open a public issue**. Instead, report it through:

- GitHub Security Advisories: https://github.com/sufield/e5s/security/advisories/new
- Email: security@sufield.dev

See [SECURITY.md](.github/SECURITY.md) for more information.

## Code Style

- Follow standard Go conventions and idioms
- Use `gofmt` or `goimports` for formatting
- Keep functions small and focused
- Write clear, descriptive variable and function names
- Add comments for exported functions and complex logic
- Prefer explicit error handling over panic
- Use early returns to reduce nesting

## Documentation

- Update README.md for user-facing changes
- Add godoc comments for exported functions and types
- Update docs/ for significant features
- Include code examples in documentation where helpful

## Testable Examples

**Important:** This project follows Go best practices for maintainable examples as described in https://go.dev/blog/example.

### How Examples Work

All example code lives in the repository and is **compiled in CI** to ensure it stays valid:

- **cmd/example-server/** - High-level API server example
- **cmd/example-client/** - High-level API client example
- **examples/middleware/** - Custom middleware patterns example

### Keeping Examples and Docs in Sync

When you change public APIs or add new features:

#### 1. Update Example Code First

Example code is the source of truth. Update the actual working examples:

```bash
# Edit the example file
vim cmd/example-server/main.go

# Verify it compiles
make build-examples

# Verify it still works (if possible)
go run ./cmd/example-server
```

#### 2. Use Example Markers

Mark important code sections with example markers so docs can reference them:

```go
// example-start:feature-name
func ImportantFeature() {
    // Your code here
}
// example-end:feature-name
```

**Existing markers:**
- `cmd/example-server/main.go`:
  - `server-setup` - Basic server configuration (lines 38-45)
  - `authenticated-endpoint` - Handler with peer identity extraction (lines 55-75)
  - `server-start` - Starting the server with e5s.Serve() (lines 77-82)

- `cmd/example-client/main.go`:
  - `client-config` - Client configuration setup (lines 70-73)
  - `client-request` - Making an mTLS request (lines 75-88)

- `examples/middleware/main.go`:
  - `auth-middleware` - Authentication middleware pattern (lines 31-45)

#### 3. Reference Examples in Documentation

Instead of copying code into docs, **reference the actual example files**:

```markdown
## Example Server

See [cmd/example-server/main.go](cmd/example-server/main.go) for a complete working example.

The key sections are marked with `example-start`/`example-end` comments:
- Server setup: lines 38-45 (marker: `server-setup`)
- Authenticated endpoint: lines 55-75 (marker: `authenticated-endpoint`)
```

**Benefits of this approach:**
- Docs always reference real, tested code
- Changes to examples automatically  propagate
- CI catches when examples break
- Users see actual working patterns

### CI Verification

CI automatically builds all examples on every push:

```bash
# What CI runs (you can run this locally too)
make build-examples
```

This ensures:
- All example code compiles
- No broken imports or syntax errors
- Examples stay valid as APIs evolve

**Location:** `.github/workflows/security.yml` contains the "Build examples" step

### When to Update Examples

Update examples when you:
- Add new public APIs
- Change existing behavior that affects usage
- Add demonstrable new features
- Fix bugs that change how code should be written

### Example Update Checklist

When updating examples:

- [ ] Update the actual example code in `cmd/` or `examples/`
- [ ] Verify it compiles: `make build-examples`
- [ ] Add or update example markers if needed
- [ ] Update line number references in documentation
- [ ] Test the example still works correctly (if possible)
- [ ] Update any related documentation that references the example

## Godoc Examples

**In addition to the runnable example programs above**, this project uses **Godoc examples** (`Example*()` functions in `*_test.go` files) to provide API documentation that appears on [pkg.go.dev](https://pkg.go.dev).

### What Are Godoc Examples?

Godoc examples are special test functions that serve as **executable documentation**. They:
- Live in `example_test.go` files alongside regular tests
- Appear automatically in package documentation on pkg.go.dev
- Are compiled (and optionally executed) by `go test`
- Follow Go's official example naming conventions

See the [Go blog post on examples](https://go.dev/blog/example) for full details.

### Where Godoc Examples Live

```
example_test.go              # High-level API examples (e5s package)
spiffehttp/example_test.go   # Low-level mTLS API examples
spire/example_test.go        # SPIRE adapter examples
```

### Example Naming Conventions

```go
func ExampleServe()              // Documents the Serve function
func ExampleServe_withConfig()   // Second example for Serve function
func ExamplePeerID()             // Documents the PeerID function
func Example_authorization()     // Package-level example demonstrating authorization
```

**Naming rules:**
- `ExampleFoo()` - Documents function/type `Foo`
- `ExampleFoo_suffix()` - Additional examples for `Foo` (suffix describes the scenario)
- `Example()` - Documents the package as a whole
- `Example_suffix()` - Package-level examples (suffix describes the use case)

### Godoc Examples vs Example Programs

**Godoc Examples** (`example_test.go`):
- **Purpose**: API documentation on pkg.go.dev
- **Audience**: Developers learning the API
- **Scope**: Individual functions/types
- **Location**: `*_test.go` files
- **Tested**: Compiled by `go test` (run if they have `// Output:` comments)

**Example Programs** (`cmd/example-*`, `examples/*`):
- **Purpose**: Complete, runnable applications
- **Audience**: Developers building similar systems
- **Scope**: Full workflows and integration patterns
- **Location**: Separate `main` packages
- **Tested**: Built by CI (`make build-examples`)

**Use both**: Godoc examples for API docs, example programs for real-world patterns.

### Writing Godoc Examples

Most Godoc examples in this project **compile but don't execute** because they require SPIRE infrastructure:

```go
// ExampleServe demonstrates starting an mTLS server.
//
// This example requires a running SPIRE agent and e5s.yaml configuration file.
func ExampleServe() {
    http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
        id, ok := e5s.PeerID(r)
        if !ok {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        fmt.Fprintf(w, "Hello, %s!\n", id)
    })

    if err := e5s.Serve(http.DefaultServeMux); err != nil {
        log.Fatal(err)
    }
    // No "// Output:" comment, so this compiles but doesn't execute
}
```

**No `// Output:` comment** means the example compiles but doesn't run. This is appropriate for:
- Code requiring external infrastructure (SPIRE, databases, etc.)
- Network operations
- Server startup code

### When to Update Godoc Examples

Update Godoc examples when you:
- **Add new exported functions or types** - Every public API should have at least one example
- **Change function signatures** - Update examples to match new parameters
- **Add new common use cases** - Add `ExampleFoo_newUseCase()` examples
- **Deprecate APIs** - Update examples to show the replacement API

### CI Testing

Godoc examples are automatically tested in CI:
- `go test ./...` compiles all examples (catches API breakage)
- Examples without `// Output:` are compile-only (appropriate for SPIRE-dependent code)
- Examples with `// Output:` are executed and output is verified

CI already runs `go test ./...` which verifies all Godoc examples compile correctly.

### Godoc Example Checklist

When adding or updating Godoc examples:

- [ ] Add example to appropriate `*_test.go` file
- [ ] Follow naming convention (`ExampleFoo` or `ExampleFoo_suffix`)
- [ ] Add clear godoc comment explaining what the example demonstrates
- [ ] Note if example requires SPIRE infrastructure
- [ ] Omit `// Output:` comment (since examples require SPIRE)
- [ ] Verify it compiles: `go test -run=^Example ./...`
- [ ] Check formatting: `gofmt -s -w .`

## License

By contributing, you agree that your contributions will be licensed under the Apache License 2.0.

## Questions?

Feel free to open an issue with the question label, or start a discussion in the GitHub Discussions tab.

## Recognition

Contributors will be recognized in:
- The project's README.md contributors section
- Release notes for significant contributions
- The project's GitHub contributors page

Thank you for contributing to e5s!
