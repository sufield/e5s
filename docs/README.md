# e5s Documentation

## Current Library

This repository provides **`pkg/identitytls`** and **`pkg/spire`** - a lightweight Go library for SPIFFE/SPIRE-based mutual TLS.

### Quick Start

**New to the library?** → See [QUICKSTART_LIBRARY.md](QUICKSTART_LIBRARY.md)

**Want a working example?** → See [examples/minikube/](../examples/minikube/)

## What's Here

- **[QUICKSTART_LIBRARY.md](QUICKSTART_LIBRARY.md)** - Library usage guide with code examples
- **[../examples/minikube/](../examples/minikube/)** - Full working demo with SPIRE cluster

## Public API

The library exposes two packages:

### `pkg/identitytls` - Core mTLS Library

Provider-agnostic primitives for building mTLS connections with SPIFFE identity:

- `NewServerTLSConfig()` - Create server TLS config with client verification
- `NewClientTLSConfig()` - Create client TLS config with server verification
- `ExtractPeerInfo()` - Extract authenticated peer identity from requests
- `CertSource` interface - Abstract certificate/trust bundle provider

### `pkg/spire` - SPIRE Adapter

SPIRE Workload API implementation of `CertSource`:

- `NewSource()` - Connect to SPIRE Agent
- Automatic certificate rotation
- Trust bundle updates
- Thread-safe, share across servers/clients

## Architecture

The old hexagonal architecture (ports, adapters, domain layer, HTTP services) has been removed. This is now a **focused library** with clean separation:

- **Core** (`pkg/identitytls`) - Defines interfaces and TLS policy
- **Adapter** (`pkg/spire`) - Implements interfaces using SPIRE

Users can implement custom `CertSource` adapters for other identity providers (Vault, cert-manager, etc.).

## Security

See [../security/](../security/) for:
- Supply chain security
- Falco runtime monitoring
- Security scanning tools

## External Resources

- [Main README](../README.md) - Project overview
- [examples/minikube/](../examples/minikube/) - Production-like demo
