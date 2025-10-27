
* Stop simulating SPIRE.
* Stop doing in-memory attestation, selector matching, in-memory CA, etc.
* Stop trying to be “SPIRE in a box.”
* Do not build a dev control plane at all.

Instead:

* Only run the parts of the library that do not depend on SPIRE (no agent, no server) in headless/tailless mode.
* Expose those parts via CLI for development and testing.
* Anything that actually needs SPIRE (identity issuance, SVIDs, workload attestation, selector matching) is not available in that mode. You don’t fake it.

That is valid, and it is the cleanest option if your priority is clarity and low surface area.

---

1. What this removes

You no longer need:

* InMemoryRegistry
* IdentityMapper / SelectorSet / MatchesSelectors()
* UnixWorkloadAttestor
* InMemoryServer that issues test certs
* InMemoryAgent that glues those together
* bootstrap_dev that wires all that
* build tags to hide the above

All of that exists only to pretend SPIRE is there when it isn’t.

You are saying: “If SPIRE isn’t there, we won’t pretend. Those features just won’t work.”

That is a perfectly valid architectural decision.

This also means:

* No separate dev library.
* No dev vs prod factories.
* No risk of leaking a fake CA into production.
* Much simpler docs.

This is a huge simplification.

---

2. What stays in the library

You keep only what is truly in scope for your library’s mission.

That includes:

A. Pure logic that does not require SPIRE
Examples (you likely already have pieces of this):

* trust domain parsing / validation (`spiffe://example.org/...` syntax)
* consistent identity representation (`WorkloadIdentity`, `IdentityCredential`, etc.)
* helper functions for logging/auditing identity in requests
* invariants / preconditions / postconditions that are enforced in-process
* request authorization decisions that operate on an *already-known* identity (not minting identity, just checking policy)
* debug/inspection commands that don’t need live SPIRE

All of those can run in headless/tailless mode through a CLI, because they’re just business logic.

B. Production integration with SPIRE

* The adapter that talks to the SPIRE Workload API via go-spiffe.
* The runtime code that extracts SPIFFE ID from the mTLS connection.
* The middleware that rejects unauthenticated requests.
* The part of the code that assumes SPIRE Agent + SPIRE Server are running.

This part requires SPIRE. You do not run it offline.

So the library ends up with two groups of capabilities:

* “SPIRE required” capabilities
* “SPIRE not required” capabilities

That’s your boundary. Not “dev vs prod.” Not build tags. Just: “does this feature inherently need SPIRE to be running?”

That boundary is simple to teach.

---

3. What headless/tailless means now

Before:

* “Headless and tailless” meant “run the app without UI or external infra, but still pretend identity exists.”

Now:

* “Headless and tailless” means “run only the side-effect-free parts of the app, with no network listeners, no external dependencies, no SPIRE.”

That aligns with hexagonal architecture in its strict reading:

* Core logic that is pure / deterministic / policy / transform can run in-process.
* Anything that depends on an external boundary (SPIRE agent socket, SPIRE server, TLS listener, OS credentials) is not part of headless mode.

This is actually closer to the spirit of hexagonal: isolate what you control (pure logic) from what you don’t (SPIRE infra).

So yes, philosophically, this is correct.

---

4. What changes in code structure

To get this result, do the following:

Step 1. Identify pure/core functionality that does not require SPIRE.
Examples (adapt to your codebase):

* trust domain parsing
* identity formatting / validation / comparison
* policy evaluation given an identity
* invariant checking and debugging helpers
* maybe simulation or dry-run of authorization decisions

Put that in a package like:

```text
internal/core/
    trustdomain.go
    identity.go
    policy.go
    diagnostics.go
```

This code:

* has zero imports of go-spiffe
* has zero socket I/O
* has zero mTLS / TLS / net/http
* has zero dependency on SPIRE Agent
* only uses Go stdlib

Step 2. Create a CLI that calls only `internal/core`
For example:

```text
cmd/cli/
    main.go
```

That CLI:

* Calls into `internal/core`
* Exposes commands like `validate-trust-domain`, `explain-identity`, `check-policy --identity spiffe://example.org/webapp --action read-db`
* Prints results
* Never tries to fetch an SVID
* Never opens a workload API socket

In other words, the CLI is an offline “introspection / reasoning / validation tool,” not an identity issuer.

Step 3. Keep SPIRE-dependent runtime code separate
For example:

```text
internal/runtime/
    spire_agent.go     // wraps go-spiffe client
    mtls_server.go     // http server that enforces SPIFFE ID
    auth_middleware.go // Extract identity from TLS connstate
```

This code:

* assumes SPIRE Agent is running
* fatal-errors / returns 401 if identity is not available
* is not callable in pure headless mode

Your production binary (e.g. `cmd/server/`) wires `internal/runtime`.

Your headless CLI binary (`cmd/cli/`) does not import `internal/runtime` at all.

Step 4. Delete all in-memory SPIRE simulation code
Remove:

* in-memory registry
* selector set types that only existed for matching
* in-memory CA
* in-memory agent
* bootstrap_dev
* dev-only factories
* build tags around these

If some of these types (like `SelectorSet`) leaked into other code, remove those dependencies. The CLI doesn’t need selector math anymore because the CLI is not pretending to be SPIRE. You don’t match selectors locally anymore at all.

Step 5. Update docs to reflect the new split
You only need two messages now:

Message for runtime (production/staging):

> This library integrates with SPIRE. It expects SPIRE Agent to be reachable over the Workload API socket. It extracts SPIFFE IDs and enforces identity-based auth in mTLS server code.

Message for CLI / headless mode:

> You can run core logic (policy checks, trust domain validation, etc.) without SPIRE. This is useful for debugging, demos, and reasoning about identity, but it does not mint or issue identities. It does not stand in for SPIRE.

That’s it. No dev vs prod story. No “fake SPIRE.” Much simpler.

---

5. Why this is better

Your simplification solves three problems cleanly:

Problem: Build tag complexity
Your fix: No build tags. You don’t ship two behaviors of the same feature. You ship fewer features offline.

Problem: Learning curve for contributors
Your fix: Newcomers only learn:

* “Here is the production story (must have SPIRE Agent).”
* “Here is the offline story (no SPIRE, but also no identity issuance).”
  They don’t have to learn selector matching internals.

Problem: Risk of accidentally shipping an in-memory CA
Your fix: There is no in-memory CA anymore, so you can’t ship it.

---

6. One important tradeoff

By making this cut, you are explicitly deciding:

* You cannot do a full “identity demo” without SPIRE running.

  * You cannot show “this process became spiffe://example.org/foo” unless SPIRE Agent + SPIRE Server are up.

That’s the cost.

If you are okay with that cost, then your simplification is correct and you should do it. You are drawing a very hard line:

> “Identity issuance is out of scope for this library unless SPIRE is present.”

That’s a clean product boundary.

---

Your new direction is valid and simpler:

* The library will not attempt to reproduce SPIRE’s control plane in dev.
* Headless/tailless mode will only run code that does not require live SPIRE.
* The CLI becomes an offline tool for the pure/core parts of the library, not an identity issuer.
* All the in-memory registry / selector matching / in-memory CA code can be deleted, instead of being hidden behind build tags or moved to a second module.

That is a sound architectural decision.
