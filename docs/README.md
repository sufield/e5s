# Documentation

This documentation follows the [Diátaxis framework](https://diataxis.fr/) for clarity and ease of navigation.

## 📚 Documentation Types

### 🎓 [Tutorials](tutorials/) - **Learning-Oriented**

*Start here if you're new to the project.*

Step-by-step introductions that teach you how to use the system through hands-on examples.

- **[Quick Start](tutorials/QUICKSTART.md)** - Get up and running in 5 minutes
- **[Editor Setup](tutorials/EDITOR_SETUP.md)** - Configure your IDE for development
- **[Prerequisites](tutorials/examples/PREREQUISITES.md)** - Essential background before running examples
- **[Examples](tutorials/examples/)** - Hands-on mTLS server and client examples

**When to use**: You want to **learn** by doing and need guided practice.

---

### 🔧 [How-To Guides](how-to-guides/) - **Task-Oriented**

*Come here when you have a specific goal to achieve.*

Practical solutions for specific tasks and problems you'll encounter in real-world usage.

**Deployment & Operations**:
- **[Production Workload API](how-to-guides/PRODUCTION_WORKLOAD_API.md)** - Deploy with kernel-level attestation
- **[Troubleshooting](how-to-guides/TROUBLESHOOTING.md)** - Debug common issues

**Development & Testing**:
- **[CodeQL Local Setup](how-to-guides/codeql-local-setup.md)** - Run security analysis locally
- **[Security Tools](how-to-guides/security-tools.md)** - Set up security scanning

**Workarounds & Fixes**:
- **[SPIRE Distroless Workaround](how-to-guides/SPIRE_DISTROLESS_WORKAROUND.md)** - Fix distroless image issues
- **[Unified Config Improvements](how-to-guides/UNIFIED_CONFIG_IMPROVEMENTS.md)** - Simplify configuration

**When to use**: You know **what** you want to do and need **how** to do it.

---

### 📖 [Reference](reference/) - **Information-Oriented**

*Look here when you need precise technical details.*

Authoritative specifications, APIs, contracts, and technical descriptions.

**Architecture Contracts**:
- **[Port Contracts](reference/PORT_CONTRACTS.md)** - Interface definitions and contracts
- **[Invariants](reference/INVARIANTS.md)** - System guarantees and assumptions
- **[Domain Model](reference/DOMAIN.md)** - Core domain types and rules

**Testing**:
- **[Test Architecture](reference/TEST_ARCHITECTURE.md)** - How tests are organized
- **[Testing Guide](reference/TESTING_GUIDE.md)** - Comprehensive testing documentation
- **[Integration Test Optimization](reference/INTEGRATION_TEST_OPTIMIZATION.md)** - Performance improvements
- **[End-to-End Tests](reference/END_TO_END_TESTS.md)** - Full system testing
- **[Property-Based Testing](reference/pbt.md)** - PBT patterns and practices

**Verification**:
- **[Verification](reference/VERIFICATION.md)** - System validation procedures

**When to use**: You need **accurate**, **complete** information about how something works.

---

### 💡 [Explanation](explanation/) - **Understanding-Oriented**

*Read these to understand the "why" behind the design.*

Background, rationale, and deep dives into design decisions and architectural choices.

**Architecture & Design**:
- **[Architecture](explanation/ARCHITECTURE.md)** - System architecture overview
- **[Architecture Review](explanation/ARCHITECTURE_REVIEW.md)** - Design decisions and trade-offs
- **[Design by Contract](explanation/DESIGN_BY_CONTRACT.md)** - Why we use contracts

**Evolution & Decisions**:
- **[SPIFFE ID Refactoring](explanation/SPIFFE_ID_REFACTORING.md)** - Why we refactored identity handling
- **[Iterations Summary](explanation/ITERATIONS_SUMMARY.md)** - Project evolution history

**Features & Patterns**:
- **[Debug Mode](explanation/DEBUG_MODE.md)** - Why and how debug mode works
- **[Refactoring Patterns](explanation/REFACTORING_PATTERNS.md)** - Common refactoring approaches

**Project Status**:
- **[Project Setup Status](explanation/PROJECT_SETUP_STATUS.md)** - Current state and roadmap

**When to use**: You want to **understand** the reasoning, history, or context behind decisions.

---

## 🗺️ Quick Navigation

### I'm a **new user**
→ Start with **[Tutorials](tutorials/)** to learn the basics

### I need to **solve a problem**
→ Check **[How-To Guides](how-to-guides/)** for practical solutions

### I need **technical details**
→ Look in **[Reference](reference/)** for specifications

### I want to **understand the design**
→ Read **[Explanation](explanation/)** for context and rationale

---

## 📊 Diátaxis Framework

This documentation structure follows the Diátaxis framework, which organizes documentation by **user needs**:

|                | **Practical** | **Theoretical** |
|----------------|---------------|-----------------|
| **Learning**   | Tutorials     | Explanation     |
| **Working**    | How-to guides | Reference       |

**Benefits**:
- ✅ Easy to find what you need based on your current goal
- ✅ Clear separation between learning, doing, and understanding
- ✅ Consistent organization across the entire project
- ✅ Reduces cognitive load when navigating documentation

Learn more about Diátaxis at [diataxis.fr](https://diataxis.fr/)

---

## 🔗 External Resources

- **[Main README](../README.md)** - Project overview and API reference
- **[Examples](tutorials/examples/)** - Hands-on code examples
- **[Contributing](#)** - How to contribute (if you have a CONTRIBUTING.md)

---

## 📝 Documentation Metadata

Each document includes a header indicating its type:

```markdown
---
type: tutorial | how-to | reference | explanation
audience: beginner | intermediate | advanced
---
```

This helps you quickly identify if a document matches your needs.

---

## 🤝 Contributing to Documentation

When adding new documentation:

1. **Identify the type**: Is it a tutorial, how-to guide, reference, or explanation?
2. **Place it correctly**: Put it in the appropriate folder
3. **Add metadata**: Include the document type header
4. **Update this index**: Add a link to the relevant section above
5. **Check links**: Ensure all cross-references work

### Decision Matrix: Where Does a New Doc Go?

**Is it teaching someone to use the system for the first time?**
→ `tutorials/`

**Is it solving a specific task or problem?**
→ `how-to-guides/`

**Is it documenting an API, contract, or specification?**
→ `reference/`

**Is it explaining why we made a design decision?**
→ `explanation/`

### Good Practices

- **Tutorials** should be complete, self-contained lessons
- **How-to guides** should focus on one specific task
- **Reference** docs should be comprehensive and precise
- **Explanations** should provide context, not instructions

---

## ❓ Still Can't Find What You Need?

- Check the **[main README](../README.md)** for an overview
- Browse **[examples/](tutorials/examples/)** for code samples
- Open an issue if documentation is missing or unclear
