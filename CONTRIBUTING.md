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

- Go 1.23 or later
- SPIRE (for integration tests)
- golangci-lint
- gosec
- govulncheck

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
