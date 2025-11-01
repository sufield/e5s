# e5s Examples

Complete examples demonstrating how to build mTLS applications with e5s and SPIRE.

---

## Quick Start

Choose the example that matches your use case:

### [📚 highlevel/](highlevel/) - Recommended Starting Point

**For**: Application developers building mTLS services
**API**: High-level, production-ready API
**Complexity**: Simple - just `e5s.Run()` and `e5s.Get()`

Complete documentation and tutorials:
- **[View All Documentation →](highlevel/TABLE_OF_CONTENTS.md)**
- **[Quick Tutorial →](highlevel/TUTORIAL.md)** - Build your first mTLS app in 30 minutes
- **[SPIRE Setup →](highlevel/SPIRE_SETUP.md)** - Set up SPIRE infrastructure in Minikube
- **[Advanced Patterns →](highlevel/ADVANCED.md)** - Production patterns and best practices

### [🔧 middleware/](middleware/)

**For**: Developers who need custom middleware integration
**API**: Middleware-based API with Chi router
**Complexity**: Moderate - more control, more code

Demonstrates:
- Custom middleware integration
- Chi router setup
- Manual identity extraction
- Request context management

### [⚙️ minikube-lowlevel/](minikube-lowlevel/)

**For**: Platform engineers and operators
**API**: Low-level SPIRE setup and infrastructure
**Complexity**: Advanced - direct SPIRE configuration

Demonstrates:
- Complete SPIRE deployment in Kubernetes
- Manual workload registration
- Infrastructure automation
- Kubernetes-native deployments

---

## Which Example Should I Use?

| Your Goal | Use This Example | Documentation |
|-----------|------------------|---------------|
| Build mTLS applications quickly | [highlevel/](highlevel/) | [Table of Contents](highlevel/TABLE_OF_CONTENTS.md) |
| Learn e5s API from scratch | [highlevel/](highlevel/) | [Tutorial](highlevel/TUTORIAL.md) |
| Set up SPIRE infrastructure | [highlevel/](highlevel/) | [SPIRE Setup](highlevel/SPIRE_SETUP.md) |
| Custom middleware integration | [middleware/](middleware/) | See middleware/README.md |
| SPIRE platform operations | [minikube-lowlevel/](minikube-lowlevel/) | See minikube-lowlevel/README.md |
| Production deployment patterns | [highlevel/](highlevel/) | [Advanced Guide](highlevel/ADVANCED.md) |
| Troubleshooting issues | [highlevel/](highlevel/) | [Troubleshooting](highlevel/TROUBLESHOOTING.md) |

---

## Getting Started

**Most users should start here:**

1. **[Set up SPIRE](highlevel/SPIRE_SETUP.md)** - Install SPIRE in Minikube (~15 minutes)
2. **[Follow the Tutorial](highlevel/TUTORIAL.md)** - Build your first mTLS app (~30 minutes)
3. **[Explore Advanced Patterns](highlevel/ADVANCED.md)** - Production-ready patterns

---

## Documentation Navigation

### For End Users
```
examples/highlevel/TABLE_OF_CONTENTS.md
    ├── TUTORIAL.md          (Start here)
    ├── SPIRE_SETUP.md       (Infrastructure setup)
    ├── README.md            (API overview)
    ├── ADVANCED.md          (Production patterns)
    └── TROUBLESHOOTING.md   (Problem solving)
```

### For Library Developers
```
examples/highlevel/
    ├── SPIRE_SETUP.md           (Infrastructure setup)
    ├── TESTING_PRERELEASE.md    (Testing local changes)
    └── TROUBLESHOOTING.md       (Debugging)
```

---

## Example Structure

```
examples/
├── README.md                    ← You are here
├── highlevel/                   ← Recommended starting point
│   ├── TABLE_OF_CONTENTS.md    ← Complete documentation index
│   ├── TUTORIAL.md              ← Step-by-step guide
│   ├── SPIRE_SETUP.md           ← Infrastructure setup
│   ├── ADVANCED.md              ← Production patterns
│   ├── TESTING_PRERELEASE.md   ← For library developers
│   ├── TROUBLESHOOTING.md       ← Problem solving
│   └── e5s.yaml                 ← Configuration template
├── middleware/                  ← Middleware integration
└── minikube-lowlevel/          ← SPIRE infrastructure
```

---

## Next Steps

**New to e5s?**
→ [View Complete Documentation](highlevel/TABLE_OF_CONTENTS.md)

**Ready to build?**
→ [Start the Tutorial](highlevel/TUTORIAL.md)

**Need SPIRE?**
→ [Set up SPIRE](highlevel/SPIRE_SETUP.md)

**Production deployment?**
→ [Read Advanced Guide](highlevel/ADVANCED.md)
