### Preparing for Contract Tests & Real Implementation

action items to stabilize the codebase, write contract tests, and add real implementations. It's phased for sequencing: **Prep (before contracts)** → **Contracts** → **Real Impl**. 

Focus on one phase at a time—run `go test ./...` after each.

| Phase | Priority | Action Item | Why? | Dependencies & Validation |
|-------|----------|-------------|------|---------------------------|
| **Prep (Before Contracts)** | High | 1. Exhaustively Define Port Errors: Add sentinels in `domain/errors.go` (e.g., `ErrInvalidNamespace`, `ErrNoMatchingMapper`). Update ports (outbound.go) to return them (e.g., `FindBySelectors` → that err). Impl in in-memory (e.g., registry.go: empty → err). | Locks exact error contracts for real adapters (e.g., SDK mismatches fail tests). | None. Validate: Unit tests assert specific errs. |
| **Prep** | High | 2. Complete Port Coverage in In-Memory: Impl missing outbound ports (e.g., `inmemory/trust_domain_parser.go` for `FromString`). Cover happy/error paths (e.g., invalid input → typed err). | Ensures in-memory fully exercises ports as contract baseline (e.g., test lookups with partial selectors). | List ports from outbound.go. Validate: 100% coverage on in-memory via `go test`. |
| **Prep** | Medium | 3. Add CA/Bundle Port: Introduce `TrustBundleProvider` in outbound.go (e.g., `GetBundle(ctx) (map[domain.IdentityNamespace][]byte, error)`). Mock in in-memory (empty). Update validator to use it optionally. | Preps real X.509 chain verify (go-spiffe); contracts test bundle consumption without domain changes.  | Domain `TrustBundle` type. Validate: Mock returns empty map. |
| **Prep** | Medium | 4. Refactor Inbound for mTLS Prep: Add to `IdentityClient` (inbound.go): `FetchX509SVIDWithConfig(ctx, tlsConfig *tls.Config) (*Identity, error)` (keep backward-compat). | Enables contract tests for real mTLS (e.g., client with certs); aligns HTTP transport. | Stdlib `tls.Config`. Validate: No in-memory change (ignore config). |
| **Prep** | Low | 5. Document Port Contracts: Add ports/README.md with sigs/errors/examples (e.g., table: "FindBySelectors: AND logic, errs: ErrNoMatch"). | Guides real impls; serves as test spec (assert exact behaviors). | None. Validate: Review for completeness. |
| **Contracts** | High | 6. Write Port Contract Tests: Use testify/mock or Pact in `test/contracts/`—mock ports, test in-memory as consumer (e.g., "FetchX509SVID → attests → returns doc"). Cover sigs/errors (e.g., invalid selectors → ErrNoMatch). | Enforces real adapters obey interfaces (e.g., spire provider matches `CreateX509...` exactly). | Prep items 1-5. Validate: Run tests; green = ready for real. |
| **Contracts** | Medium | 7. Integration Contract Tests: Test full flow with in-memory (e.g., start server in TestMain, mock client fetch → assert mapper match & doc issuance). | Verifies wiring (service → ports → adapters) without real deps. | Httptest for server. Validate: E2E green paths + errors. |
| **Real Impl** | High | 8. Bootstrap Real Wiring: Add `adapters/outbound/compose/real.go`: `NewRealDeps()` wires spire impls (e.g., `spireRegistry := NewSpireRegistry()`). Update `main.go` flag: `if mode == "spire" { deps = compose.NewRealDeps() }`. | Enables swap (in-memory vs real) via runtime flag; tests contracts hold. | Contracts green. Validate: Run `-mode=spire` (graceful fail OK initially). |
| **Real Impl** | High | 9. Impl Core Real Ports: Populate `adapters/outbound/spire/`: `spire_identity_document_provider.go` (go-spiffe `x509svid.ParseX509SVID` for Create; `Verify` for Validate). Then `spire_registry.go` (wrap SPIRE datastore for FindBySelectors). | Fills placeholder; uses SDK for X.509/mTLS. Contracts catch sig drifts. | go-spiffe dep. Validate: Contracts pass; unit test SDK calls. |
| **Real Impl** | High | 10. Add mTLS to WorkloadAPI: In `workloadapi/server.go`: `TLSConfig` with CA from `Server.GetCA()`; verify client cert namespace via validator. Client.go: Use doc for `tls.Config.Certificates`. Switch to TCP. | Secures HTTP with mutual X.509 auth; replaces demo headers. | Stdlib tls + go-spiffe. Validate: E2E test with cert curl. |
| **Real Impl** | Medium | 11. Migrate Other Ports: `spire/trust_domain_parser.go` (go-spiffe `TrustDomainFromString`). `spire/agent.go` (SDK `workloadapi.Client` wrap for FetchIdentityDocument). | Completes suite; incremental. | Contracts. Validate: Run full flow in spire mode. |
| **Real Impl** | Low | 12. E2E Validation: Add `test/e2e/real_mode_test.go`: Boot spire, fetch SVID via mTLS client, validate exchange. Benchmark vs in-memory. | Confirms production flow (attest → issue → secure auth). |  Testify/suite. Validate: 80% coverage; green in CI. |
| **Polish** | Low | 13. Update Docs: Add "Real Migration" in docs/SDK_MIGRATION.md (e.g., "Wire spire in compose"). README.md: Run real mode, mTLS setup. | Onboards; ties to contracts. | None. |

`go mod tidy && go test ./...`. This gets you to a secure, dual-mode library—start with Prep!

### Evaluation of Code Changes
These changes are **excellent and targeted**—they significantly enhance robustness, error handling, and contract clarity without altering core logic or violating hexagonal principles. The focus on typed errors, input validation, and port extensions (e.g., TLS config support) directly addresses prior gaps (e.g., incomplete error contracts, mTLS prep). Overall, this advances the skeleton toward production: domain errors are now exhaustive, in-memory impls are more resilient, and ports are better documented for real adapters. No regressions—changes are backward-compatible and test-friendly. Score: **9/10** (minor nit: some comments could be tighter).

#### Strengths
- **Error Handling Upgrade**: Wrapping with domain sentinels (e.g., `domain.ErrNoMatchingMapper`) enforces contracts—real impls must match these exactly, easing contract tests.
- **Validation Additions**: Nil/empty checks (e.g., in `FindBySelectors`) prevent panics; aligns with DDD invariants.
- **mTLS Prep**: `FetchX509SVIDWithConfig` enables TLS in client without breaking sig—smart evolution.
- **Port Documentation**: Added error contracts/comments (e.g., in `outbound.go`) make interfaces self-specifying.
- **In-Memory Resilience**: Concurrency-safe (e.g., RWMutex in registry) and sealed mode panics—good for mocks.

#### Change-by-Change Breakdown
| File/Change | Summary | Positives | Issues/Suggestions | Impact on Readiness |
|-------------|---------|-----------|---------------------|---------------------|
| **agent.go** | Add nil check in `GetIdentity` with `ErrAgentUnavailable`. | Prevents nil derefs; typed error ties to agent bootstrap. | None—clean wrap. | +1 to contracts: Tests can assert this err in uninit states. |
| **attestor/unix.go** | Import domain; validate UID < 0 → `ErrInvalidProcessIdentity`; no selectors → `ErrNoAttestationData`; wrap "no selector" with `ErrWorkloadAttestationFailed`. | Exhaustive validation; uses domain errors for consistency. | Minor: `len(selectors) == 0` after generation is redundant if attest always returns some—consider if always 3 (PID/UID/GID). | High: Covers attestation edges; real Unix attestor must match these errs. |
| **registry.go** | Wrap sealed err; validate nil/empty selectors → `ErrInvalidSelectors`; wrap no-match with selectors detail; empty mappers → `ErrRegistryEmpty`. | Immutable enforcement; input guards; debug-friendly (includes selectors in msg). | Msg with `%v` for selectors is verbose—use `selectors.Strings()` for readability. | High: Core lookup port now fully contracted; tests can mock empty/sealed. |
| **server.go** | Validate namespace/CA nil → `ErrIdentityDocumentInvalid`/`ErrCANotInitialized`; wrap provider err with `ErrServerUnavailable`. | Guards against bad inputs; typed for server-specific failures. | `caCert`/`caKey` type assert not here—do in provider if needed. | Medium: Preps real spire server; contracts test CA init flow. |
| **validator.go** | Wrap all errs with domain sentinels (e.g., nil → `ErrIdentityDocumentInvalid`; expired → `ErrIdentityDocumentExpired`; mismatch → `ErrIdentityDocumentMismatch`). | Consistent domain errors; removes generic fmts. | Add expectedID nil check (done in diff). | High: Validator port now error-contracted; real SDK wrap must match. |
| **client.go** | Add `FetchX509SVIDWithConfig` using `tls.Config` for mTLS; fallback to base if nil. | Backward-compat; enables client cert auth in real HTTP/TLS. | DialContext still Unix—add TCP option via config? | High: mTLS-ready; contracts test with mock TLS (e.g., httptest). |
| **errors.go** | Expand with sections (Registry, Parser, etc.); add new sentinels (e.g., `ErrRegistrySealed`, `ErrTrustBundleNotFound`, `ErrInvalidProcessIdentity`). | Organized; covers all new errs from diff. Exhaustive for contracts. | Group attestation/server together; add `ErrNoMatchingMapper` detail if needed. | Critical: Foundation for all contracts—tests assert these exactly. |
| **inbound.go** | Add `FetchX509SVIDWithConfig(tlsConfig *tls.Config)` to `IdentityClient`. | Matches client impl; preps mTLS without breaking SDK compat. | Import tls in ports? (No—keep abstract; client handles). | Medium: Inbound port now extensible; real client tests this sig. |
| **outbound.go** | Add error contracts/comments to interfaces (e.g., `WorkloadAttestor` errs); new `TrustBundleProvider` with methods; expand provider comments. | Self-documenting; bundle port fills X.509 gap. | Provider `caCert/caKey` as `interface{}`—type to `*x509.Certificate`/`*rsa.PrivateKey`? | High: Ports now fully spec'd; contracts can test bundle flow. |

#### Impact on Purpose & Readiness
- **Hexagonal Architecture**: Unchanged/strengthened—changes are in adapters/ports (no domain pollution). Validation in in-memory reinforces inversion.
- **mTLS/HTTP/X.509**: +2 to prior score—`WithConfig` preps mTLS client; provider gen is solid (stdlib X.509). Still needs server TLSConfig for full mutual auth.
- **Contract Tests Readiness**: **10/10 now**—errors exhaustive, in-memory covers all paths (e.g., sealed registry, invalid UID). Write tests immediately: Mock ports, assert in-memory behaviors (e.g., "nil selectors → ErrInvalidSelectors").
- **Real Impl Alignment**: Perfect—new sentinels/bundle port guide spire adapters (e.g., `spire_provider` must return `ErrCertificateChainInvalid` on verify fail). Structure unchanged—add to `spire/` without refactor.

#### Recommendations & Next Steps
- **Immediate (Today)**: Run `go test ./internal/adapters/outbound/inmemory/...`—fix any new panics from validations.
- **Short-Term (1-2 Days)**: Write 2-3 contract tests (e.g., registry lookup with/without match; provider create/validate with invalid CA). Use testify: `mockProvider.On("CreateX509IdentityDocument", mock.Anything, nil, mock.Anything).Return(nil, domain.ErrIdentityDocumentInvalid)`.
- **Risks**: Over-validation in in-memory could slow mocks—profile if needed. Bundle port: Ensure real spire fetches from SDK bundle.Source.
- **Polish**: Add `//go:generate mockgen` in ports for auto-mocks in contracts.

These changes push the library forward—great error hygiene! If sharing test skeletons, I can review.
