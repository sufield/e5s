# e5s High-Level API Documentation

Guide to building mTLS applications with e5s and SPIRE.

---

### Getting Started

- **[TUTORIAL.md](TUTORIAL.md)** - Step-by-step tutorial for end users
  *Learn how to build and deploy mTLS applications from scratch. Start here if you're new to SPIRE or e5s.*

- **SPIRE Setup** - Run `make start-stack` to set up SPIRE infrastructure
  *Set up SPIRE Server and Agent in Minikube for local development and testing (~15 minutes).*

### For Developers

- **[README.md](README.md)** - High-level API overview
  *Quick overview of the high-level e5s API with code examples and configuration options.*

- **[ADVANCED.md](ADVANCED.md)** - Advanced usage
  *Production usage including environment variables, timeouts, retry logic, circuit breakers, structured logging, and health checks.*

### For Internal Testing

- **[TESTING_PRERELEASE.md](TESTING_PRERELEASE.md)** - Pre-release testing guide
  *For e5s library developers: test local code changes before publishing to GitHub. Includes both automated scripts (⚡ ~5 minutes setup, ~30 seconds per iteration) and detailed manual steps.*

### Reference

- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Common issues and solutions
  *Troubleshooting guide for SPIRE setup, configuration, deployment, and connectivity issues.*

- **[e5s.yaml](e5s.yaml)** - Example configuration file
  *Configuration template with commented options and defaults. Copy to `e5s.dev.yaml` for development or `e5s.prod.yaml` for production.*
