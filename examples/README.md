# e5s Examples

Examples demonstrating how to build mTLS applications with e5s and SPIRE.

## Quick Start

**New to e5s?** Start with the [High-Level API Tutorial](highlevel/TUTORIAL.md).

**Want to see middleware patterns?** Check out [middleware examples](middleware/).

**Need production deployment?** See [minikube-lowlevel](minikube-lowlevel/) for Kubernetes.

## Examples Overview

| Example | Level | Description | Use Case |
|---------|-------|-------------|----------|
| **[highlevel/](highlevel/)** | Beginner | Config-driven mTLS servers and clients | Production applications using `e5s.Start()` and `e5s.Client()` |
| **[basic-server/](basic-server/)** | Beginner | Simple mTLS server with Chi router | Quick start, testing, development |
| **[basic-client/](basic-client/)** | Beginner | Simple mTLS client | Quick start, testing, development |
| **[middleware/](middleware/)** | Intermediate | HTTP middleware patterns | Custom authentication, authorization, and monitoring |
| **[container-server/](container-server/)** | Intermediate | Dockerized mTLS server | Containerized deployments |
| **[container-client/](container-client/)** | Intermediate | Dockerized mTLS client | Containerized deployments |
| **[minikube-lowlevel/](minikube-lowlevel/)** | Advanced | Full Kubernetes deployment | Production-like infrastructure setup |

## Detailed Guides

### [highlevel/](highlevel/) - Recommended Starting Point

**For**: Application developers building mTLS services
**API**: High-level, production-ready API
**Complexity**: Simple - just `e5s.Start()` and `e5s.Client()`

**Learn**:
- [ADVANCED.md](highlevel/ADVANCED.md) - Production patterns, timeouts, retry logic
- [DEVELOPER_API.md](highlevel/DEVELOPER_API.md) - API reference

### [middleware/](middleware/)

**For**: Developers who need custom middleware integration
**API**: Middleware-based API with Chi router
**Complexity**: Moderate - more control, more code

**Demonstrates**:
- Custom middleware integration
- Chi router setup
- Manual identity extraction
- Request context management
- Trust domain enforcement
- Certificate expiry monitoring

### [container-server/](container-server/) & [container-client/](container-client/)

**For**: DevOps engineers deploying containerized applications
**API**: High-level API in Docker containers
**Complexity**: Intermediate - requires Docker knowledge

**Demonstrates**:
- Dockerizing e5s applications
- Sharing SPIRE agent socket with containers
- Container networking for mTLS

### [basic-server/](basic-server/) & [basic-client/](basic-client/)

**For**: Developers getting started quickly
**API**: High-level API with simple CLI interface
**Complexity**: Beginner - minimal setup required

**Demonstrates**:
- Simple server with Chi router and health check
- Simple client with configurable server URL
- Version information and debug modes
- CLI flag parsing patterns

### [minikube-lowlevel/](minikube-lowlevel/)

**For**: Platform engineers and operators
**API**: Low-level SPIRE setup and infrastructure
**Complexity**: Advanced - direct SPIRE configuration

**Demonstrates**:
- Complete SPIRE deployment in Kubernetes
- Manual workload registration
- Infrastructure automation
- Kubernetes-native deployments

## Running Examples

### Prerequisites

Most examples require:
- **Go 1.25+** for building Go code
- **SPIRE Agent** running locally or in container/cluster

Many examples will auto-start SPIRE for you.

### High-Level API Examples

```bash
cd highlevel

# Follow the tutorial
cat TUTORIAL.md

# Or run directly
go run main.go
```

### Basic Server and Client Examples

```bash
# Build and run the server
go run ./examples/basic-server/ -config examples/highlevel/e5s-server.yaml

# In another terminal, run the client
go run ./examples/basic-client/ \
  -e5s-config examples/highlevel/e5s-client.yaml \
  -url https://localhost:8443/time

# With debug mode
go run ./examples/basic-server/ -config examples/highlevel/e5s-server.yaml -debug
```

### Middleware Examples

```bash
cd middleware

# Run the server (SPIRE required)
go run main.go

# Test endpoints
curl https://localhost:8443/api/hello
```

### Container Examples

```bash
cd container-server

# Build and run with Docker
docker build -t e5s-server .
docker run --network=host \
  -v /path/to/spire-agent/socket:/spire-agent \
  e5s-server
```

### Kubernetes Examples

```bash
cd minikube-lowlevel

# Full deployment guide
cat deploy/README.md
```

## Learning Paths

### Path 1: Application Developer

1. Read [highlevel/TUTORIAL.md](highlevel/TUTORIAL.md) - Learn the basics
2. Study [highlevel/ADVANCED.md](highlevel/ADVANCED.md) - Production patterns
3. Review [middleware/](middleware/) - Custom authentication
4. Deploy with [minikube-lowlevel/](minikube-lowlevel/) - Production setup

### Path 2: DevOps/Platform Engineer

1. Start with [minikube-lowlevel/](minikube-lowlevel/) - Infrastructure setup
2. Review [container-server/](container-server/) and [container-client/](container-client/) - Containerization
3. Check [highlevel/](highlevel/) - Application patterns

### Path 3: Security Engineer

1. Review [middleware/](middleware/) - Authentication patterns
2. Study [highlevel/ADVANCED.md](highlevel/ADVANCED.md) - Security best practices
3. Analyze [minikube-lowlevel/](minikube-lowlevel/) - Infrastructure security

## Additional Resources

### Documentation

- [../TESTING.md](../TESTING.md) - Testing guide (unit, integration, container tests)
- [../docs/explanation/](../docs/explanation/) - Architecture and design decisions
- [../docs/how-to/](../docs/how-to/) - Task-specific guides
- [../docs/reference/](../docs/reference/) - API reference and troubleshooting

## Getting Help

- **Troubleshooting:** See [../docs/reference/troubleshooting.md](../docs/reference/troubleshooting.md)
- **Issues:** Report bugs at GitHub issues
- **Questions:** Ask in GitHub Discussions
