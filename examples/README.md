# e5s Examples

Complete examples demonstrating how to build mTLS applications with e5s and SPIRE.

---

## Quick Start

Choose the example that matches your use case:

### [📚 highlevel/](highlevel/) - Recommended Starting Point

**For**: Application developers building mTLS services
**API**: High-level, production-ready API
**Complexity**: Simple - just `e5s.Run()` and `e5s.Get()`

**Documentation**: **[View All Guides →](highlevel/TABLE_OF_CONTENTS.md)**

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

| Your Goal | Use This Example |
|-----------|------------------|
| Build mTLS applications | [highlevel/](highlevel/) → [Start Here](highlevel/TABLE_OF_CONTENTS.md) |
| Custom middleware | [middleware/](middleware/) |
| SPIRE infrastructure | [minikube-lowlevel/](minikube-lowlevel/) |

---

## Example Structure

```
examples/
├── README.md                ← You are here
├── highlevel/               ← Start here (recommended)
│   └── TABLE_OF_CONTENTS.md   ← Complete documentation index
├── middleware/              ← Custom middleware integration
└── minikube-lowlevel/      ← SPIRE infrastructure setup
```

---

**→ [View All Documentation](highlevel/TABLE_OF_CONTENTS.md)**
