# e5s Examples

Examples demonstrating how to build mTLS applications with e5s and SPIRE.

---

## Quick Start

Choose the example that matches your use case:

### [highlevel/](highlevel/) - Recommended Starting Point

**For**: Application developers building mTLS services
**API**: High-level, production-ready API
**Complexity**: Simple - just `e5s.Start()` and `e5s.Client()`

### [middleware/](middleware/)

**For**: Developers who need custom middleware integration
**API**: Middleware-based API with Chi router
**Complexity**: Moderate - more control, more code

Demonstrates:
- Custom middleware integration
- Chi router setup
- Manual identity extraction
- Request context management

### [minikube-lowlevel/](minikube-lowlevel/)

**For**: Platform engineers and operators
**API**: Low-level SPIRE setup and infrastructure
**Complexity**: Advanced - direct SPIRE configuration

Demonstrates:
- Complete SPIRE deployment in Kubernetes
- Manual workload registration
- Infrastructure automation
- Kubernetes-native deployments
