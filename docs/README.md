# e5s Documentation

## Overview

e5s provides **two APIs** for SPIFFE/SPIRE-based mutual TLS, serving different developer needs:

### High-Level API (`e5s.Start`, `e5s.Client`)

**For application developers** - Config-driven, one-line setup:
- See [../README.md](../README.md) for quick examples
- See [../examples/highlevel/](../examples/highlevel/) for production-ready server with chi router

### Low-Level API (`pkg/spiffehttp`, `pkg/spire`)

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

### `pkg/spiffehttp` - Core mTLS Library

Provider-agnostic primitives for building mTLS connections with SPIFFE identity:

- `NewServerTLSConfig()` - Create server TLS config with client verification
- `NewClientTLSConfig()` - Create client TLS config with server verification
- `PeerFromRequest()` - Extract authenticated peer identity from requests
- `PeerFromContext()` - Retrieve peer from request context
- `WithPeer()` - Attach peer to context
### `pkg/spire` - SPIRE Adapter

SPIRE Workload API client:

- `NewSource()` - Connect to SPIRE Agent
- `X509Source()` - Access underlying SDK source
- Automatic certificate rotation
- Trust bundle updates
- Thread-safe, share across servers/clients

## Architecture

This has:

- **Core** (`pkg/spiffehttp`) - TLS configuration using go-spiffe SDK
- **Adapter** (`pkg/spire`) - SPIRE Workload API client

## Security

See [../security/](../security/) for:
- Supply chain security
- Falco runtime monitoring
- Security scanning tools

## External Resources

- [Main README](../README.md) - Project overview
- [examples/minikube/](../examples/minikube/) - Production-like demo
