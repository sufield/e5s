# e5s Documentation

Documentation for the e5s SPIFFE/SPIRE mTLS library.

## Getting Started

- [Project README](../README.md) - Overview, installation, quick start
- [Examples](../examples/) - Working code for all use cases

## Tutorials

Learning-oriented guides for getting started:

- [Build Your First mTLS App](../examples/highlevel/TUTORIAL.md) - Step-by-step tutorial

## How-To Guides

Task-oriented guides for specific scenarios:

- [Deploy to Kubernetes with Helm](how-to/deploy-helm.md)
- [Debug mTLS Connections](how-to/debug-mtls.md)
- [Run Integration Tests](how-to/run-integration-tests.md)
- [Monitor with Falco](how-to/monitor-with-falco.md)
- [Falco Deep Dive](how-to/falco-guide.md)

## Explanation

Understanding e5s concepts and design:

- [Core Concepts](explanation/concepts.md) - High-level vs low-level APIs, SPIFFE/SPIRE
- [Architecture](explanation/architecture.md) - Design decisions and internal structure
- [Comparison with go-spiffe SDK](explanation/comparison.md)
- [FAQ](explanation/faq.md)
- [Design Document](explanation/design.md)

## Reference

Technical specifications and lookups:

- [Low-Level API Guide](reference/api.md) - `spire` and `spiffehttp` packages
- [Configuration Reference](reference/config.md) - Covers e5s.yaml options
- [Troubleshooting](reference/troubleshooting.md) - Error messages and solutions
