# e5s High-Level API Documentation

Complete guide to building mTLS applications with e5s and SPIRE.

---

## Quick Navigation

### Getting Started

- **[TUTORIAL.md](TUTORIAL.md)** - Step-by-step tutorial for end users
  *Learn how to build and deploy mTLS applications from scratch. Start here if you're new to SPIRE or e5s.*

- **[SPIRE_SETUP.md](SPIRE_SETUP.md)** - SPIRE infrastructure setup guide
  *Set up SPIRE Server and Agent in Minikube for local development and testing (~15 minutes).*

### For Developers

- **[README.md](README.md)** - High-level API overview
  *Quick overview of the high-level e5s API with code examples and configuration options.*

- **[ADVANCED.md](ADVANCED.md)** - Advanced usage patterns
  *Production patterns including environment variables, timeouts, retry logic, circuit breakers, structured logging, and health checks.*

### For Internal Testing

- **[QUICK_START_PRERELEASE.md](QUICK_START_PRERELEASE.md)** - ⚡ Fast 3-step testing workflow
  *Automated scripts for quick testing (~5 minutes setup, ~30 seconds per iteration).*

- **[TESTING_PRERELEASE.md](TESTING_PRERELEASE.md)** - Complete pre-release testing guide
  *For e5s library developers: test local code changes before publishing to GitHub. Includes both automated and manual workflows.*

### Reference

- **[TROUBLESHOOTING.md](TROUBLESHOOTING.md)** - Common issues and solutions
  *Troubleshooting guide for SPIRE setup, configuration, deployment, and connectivity issues.*

- **[e5s.yaml](e5s.yaml)** - Example configuration file
  *Production-ready configuration template with commented options and defaults.*

---

## Documentation Paths

### For End Users (Learning & Building)

```
TUTORIAL.md → README.md → ADVANCED.md
    ↓
SPIRE_SETUP.md (if needed)
    ↓
TROUBLESHOOTING.md (as needed)
```

**Path**: Start with the tutorial, explore the API overview, then dive into advanced patterns.

### For e5s Library Developers (Internal Testing)

**Quick Path** (Automated):
```
SPIRE_SETUP.md → QUICK_START_PRERELEASE.md (3 commands)
    ↓
TROUBLESHOOTING.md (as needed)
```

**Detailed Path** (Manual):
```
SPIRE_SETUP.md → TESTING_PRERELEASE.md
    ↓
TROUBLESHOOTING.md (as needed)
```

**Recommendation**: Use the Quick Start for fast iterations. Use the detailed guide if you need to understand each step.

---

## File Descriptions

| File | Audience | Purpose | Time |
|------|----------|---------|------|
| [TUTORIAL.md](TUTORIAL.md) | End users | Complete walkthrough from zero to working mTLS application | ~30 min |
| [SPIRE_SETUP.md](SPIRE_SETUP.md) | All users | SPIRE infrastructure setup in Minikube | ~15 min |
| [README.md](README.md) | All users | API overview and quick reference | ~10 min |
| [ADVANCED.md](ADVANCED.md) | Experienced users | Production patterns and best practices | ~20 min |
| [QUICK_START_PRERELEASE.md](QUICK_START_PRERELEASE.md) | Library developers | ⚡ Automated pre-release testing (3 commands) | ~5 min |
| [TESTING_PRERELEASE.md](TESTING_PRERELEASE.md) | Library developers | Detailed pre-release testing workflow | ~20 min |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md) | All users | Problem solving and debugging | Reference |
| [e5s.yaml](e5s.yaml) | All users | Configuration template | Reference |

---

## Next Steps

**New to e5s and SPIRE?**
→ Start with [TUTORIAL.md](TUTORIAL.md)

**Need SPIRE infrastructure?**
→ Follow [SPIRE_SETUP.md](SPIRE_SETUP.md)

**Testing library changes?**
→ ⚡ [QUICK_START_PRERELEASE.md](QUICK_START_PRERELEASE.md) (3 commands, ~5 min)
→ Or [TESTING_PRERELEASE.md](TESTING_PRERELEASE.md) (detailed guide)

**Stuck on an issue?**
→ Check [TROUBLESHOOTING.md](TROUBLESHOOTING.md)

**Building for production?**
→ Read [ADVANCED.md](ADVANCED.md)
