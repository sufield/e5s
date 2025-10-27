# ADR: Scope of Domain Logic vs SPIRE Integration

## Status

✅ Accepted

## Decision

The library will:

* Keep only domain logic that can run and be tested in isolation.
* Treat SPIRE (Agent + Server) as an external dependency that the library integrates with, but does not replicate.
* Remove (or avoid introducing) any in-memory SPIRE substitute logic (local registry, selector matching, in-memory CA, fake Agent).
* Expose a CLI / headless mode that exercises only domain logic and does not require SPIRE.

## Motivation

Hexagonal architecture is not about reimplementing infrastructure.
It is about isolating **business rules** from **infrastructure concerns** so that:

1. The business rules can be tested without infrastructure.
2. Infrastructure-specific adapters can change without forcing changes to the business rules.

In this project:

* The “business rules” we care about are local, in-process rules we own and control.
* SPIRE’s identity issuance, selector matching, and attestation are infrastructure concerns we do **not** own.
* Replicating SPIRE in-process just to make dev “nicer” crosses the boundary and increases accidental complexity.

We are choosing to keep that boundary strict.

## What that means concretely

### 1. The domain layer stays

The domain layer should include:

* Trust domain rules (parsing, validation, formatting, invariants)
* Identity value objects (e.g. `WorkloadIdentity`, `IdentityCredential`, `IdentityDocument` if it represents an issued identity, not the issuance process)
* Authorization / policy decisions that take an identity and decide “allow / deny”
* Auditing/enrichment helpers (turn identity into structured metadata for logs / tracing)

All of that:

* Must be pure.
* Must be easily unit-testable.
* Must never reach out to sockets, file descriptors, SPIRE, or the OS.
* Must have deterministic behavior.

This is what hexagonal architecture is buying us:

* We can write fast, isolated tests with no SPIRE Agent, no SPIRE Server, no network.

And importantly:

* This logic is worth maintaining in our codebase.
* This logic is part of the value we ship.

### 2. SPIRE is an adapter, not the domain

The SPIRE-dependent pieces are infrastructure adapters:

* Calling `go-spiffe` to fetch the current workload SVID.
* Extracting SPIFFE ID from an mTLS connection.
* Verifying peer identity using SPIRE trust bundles.

Those belong in a runtime adapter package like `runtime/spireagent` or `adapters/spire`.

Key property:

* That code expects SPIRE to be up.
* It is not callable in unit tests without SPIRE.
* It is not runnable in offline/headless CLI mode.
* It is allowed to depend on `go-spiffe` and OS details.

Hexagonal read:

* Domain defines “what is an identity and how do we reason about it.”
* Adapter defines “how we obtain that identity in this deployment.”

We keep that separation.

### 3. The CLI/headless mode only runs domain logic

The CLI is allowed to do things like:

* Validate that a SPIFFE ID belongs to a given trust domain.
* Show how an identity would be logged / audited.
* Run authorization decisions for hypothetical identities.
* Inspect config / invariants.

The CLI is **not** allowed to:

* Mint identities.
* Pretend to be an Agent.
* Pretend to be a Server.
* Do attestation and selector matching locally.
* Issue SVID-like certs from an in-memory CA.

Why?
Because those behaviors are SPIRE’s job.
If we fake them, we are:

* Re-implementing infrastructure.
* Increasing surface area.
* Creating a second mental model contributors must learn.
* Risking accidental production misuse.

By not faking them, we reduce code volume and cognitive load.
New contributors do not need to learn “our mini-SPIRE.”
They learn:

1. Domain logic we actually own.
2. How we plug into SPIRE in production.

This directly matches your statement:

> "The core value provided by hexagonal architecture is extracting the domain logic and test it in isolation without any dependencies on the server. This option provides that value and reduces the amount of code needed and complexity of the learning curve."

Correct.

### 4. Implications for code cleanup

To fully reflect this decision, you should:

* Remove (or not add) code that simulates SPIRE control plane behavior:

  * in-memory registry for selector → SPIFFE ID mapping
  * selector matching logic (`MatchesSelectors()`, AND logic across selectors)
  * unix attestor that inspects PID/UID and fabricates selectors
  * in-memory CA that issues certs
  * “InMemoryAgent” that glues all of the above
  * any bootstrap code whose job is to assemble that fake control plane
* Remove build tags whose only purpose was to hide that fake control plane in production builds

After cleanup:

* There is no “dev Agent,” only “Prod Agent,” which talks to SPIRE.
* If SPIRE is not running, identity-dependent features are simply unavailable. We do not degrade into a fake path.

This is simpler for contributors:

* They do not have to learn two paths for identity.
* They do not have to understand selector matching internals.
* They only have to understand:

  * pure domain logic (testable anywhere)
  * and the SPIRE adapter (which requires SPIRE to run)

### 5. How testing works after this decision

Unit tests:

* Target the domain layer directly.
* Run without SPIRE.
* Prove that once you *have* an identity, your logic does the right thing (authorization, trust domain checks, formatting, etc.).

Integration tests:

* Spin up SPIRE (Agent + Server) and exercise the adapter.
* Prove that we can actually obtain a real SPIFFE ID over the Workload API and bind it into our request handling.

No “fake SPIRE” tests.
No “fake CA.”
No “selector matching emulation.”

That removes a huge category of test maintenance.

## Result

This decision gives you:

* Less code to write and keep correct.
* Fewer abstractions to teach.
* No build tags.
* A library that mirrors production reality instead of simulating it.
* A domain layer that is pure, testable, and demonstrably independent of SPIRE — which is exactly what hexagonal architecture is supposed to deliver.

And that matches your goal:

* Keep the value of hexagonal (test the domain in isolation).
* Avoid dragging SPIRE’s internal control plane concerns into your codebase.
* Lower the learning curve for anyone using or contributing to the library.

This is the right cut.
