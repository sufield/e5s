# API Reference

Complete API documentation for e5s is available on pkg.go.dev:

**https://pkg.go.dev/github.com/sufield/e5s**

The generated documentation includes:
- All exported functions, types, and constants
- Function signatures and return values
- Usage examples from package comments
- Cross-referenced type definitions
- Source code links

## Package Overview

| Package | Purpose | Documentation |
|---------|---------|---------------|
| **`e5s`** | High-level config-driven API | [pkg.go.dev/github.com/sufield/e5s](https://pkg.go.dev/github.com/sufield/e5s) |
| **`spire`** | SPIRE Workload API client | [pkg.go.dev/github.com/sufield/e5s/spire](https://pkg.go.dev/github.com/sufield/e5s/spire) |
| **`spiffehttp`** | Provider-agnostic mTLS primitives | [pkg.go.dev/github.com/sufield/e5s/spiffehttp](https://pkg.go.dev/github.com/sufield/e5s/spiffehttp) |

## Viewing Locally

```bash
# View package-level documentation
go doc github.com/sufield/e5s

# View specific function
go doc github.com/sufield/e5s.Start
go doc github.com/sufield/e5s.Client

# View all exported symbols
go doc -all github.com/sufield/e5s
```

Or run a local documentation server:

```bash
# Install godoc
go install golang.org/x/tools/cmd/godoc@latest

# Start server
godoc -http=:6060

# Visit http://localhost:6060/pkg/github.com/sufield/e5s/
```
