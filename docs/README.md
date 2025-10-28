# e5s Documentation

## Overview

e5s provides **two APIs** for SPIFFE/SPIRE-based mutual TLS, serving different developer needs:

### High-Level API (`e5s.Start`, `e5s.Client`)

**For application developers** - Config-driven, one-line setup:
- See [../README.md](../README.md) for quick examples
- See [../examples/highlevel/](../examples/highlevel/) for production-ready server with chi router

### Low-Level API (`pkg/identitytls`, `pkg/spire`)

**For platform/infrastructure teams** - Full control over TLS and identity:
- See [QUICKSTART_LIBRARY.md](QUICKSTART_LIBRARY.md) for API reference
- See [../examples/minikube-lowlevel/](../examples/minikube-lowlevel/) for SPIRE cluster setup

## Quick Start

**New to e5s?** → Start with the [high-level API](../README.md#high-level-api-e5sstart-e5sclient) in the main README

**Need full control?** → See [QUICKSTART_LIBRARY.md](QUICKSTART_LIBRARY.md) for low-level API usage

## What's Here

- **[QUICKSTART_LIBRARY.md](QUICKSTART_LIBRARY.md)** - Low-level API reference with code examples
- **[../examples/highlevel/](../examples/highlevel/)** - High-level API example (application developers)
- **[../examples/minikube-lowlevel/](../examples/minikube-lowlevel/)** - Low-level API example with full SPIRE cluster (platform teams)

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
