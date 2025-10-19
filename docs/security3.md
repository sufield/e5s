Looks solid and security-focused. A few tweaks will make it cleaner, faster, and closer to “secure by default.”

## High-impact fixes

1. **Build tags scope**

   * You’ve set `run.build-tags: [dev]`. That means prod-only files (no tag) won’t be linted. Either:

     * remove the line (lint default build), or
     * run **two** lint jobs in CI: one default, one with `--build-tags=dev`.

2. **Import grouping**

   * Add a `goimports.local-prefixes` so imports from your module are grouped/stable:

     * `local-prefixes: github.com/pocket/hexagon/spire`

3. **`//nolint` hygiene**

   * Add `nolintlint` to prevent blanket/no-reason suppressions:

     * enforce reasons and specific linters.

4. **Disallow stray prints in library code**

   * Add `forbidigo` to block `fmt.Print*` (allow in `cmd/` and tests via exclude rule).

5. **Misspell locale**

   * Set `misspell.locale: US` to avoid UK/US noise.

6. **gofmt simplifications**

   * Turn on `gofmt.simplify: true`.

## Optional (policy-level)

* **depguard**: disallow risky or non-approved deps (e.g., direct `crypto/tls` in library code if you want all TLS funneled through your adapter). If that’s too opinionated for a demo, skip it.

## Suggested minimal patch

```yaml
run:
  timeout: 5m
  tests: true
  # Consider removing this OR run a second job with the dev tag:
  # build-tags:
  #   - dev

linters:
  enable:
    - gosec
    - govet
    - staticcheck
    - errcheck
    - gocritic
    - revive
    - ineffassign
    - unused
    - bodyclose
    - gofmt
    - goimports
    - misspell
    - unconvert
    - unparam
    - nolintlint      # NEW
    - forbidigo       # NEW
    # - depguard      # Optional (policy)

linters-settings:
  gofmt:
    simplify: true

  goimports:
    local-prefixes: github.com/pocket/hexagon/spire

  misspell:
    locale: US

  nolintlint:
    allow-leading-space: false
    allow-unused: false
    require-explanation: true
    require-specific: true

  forbidigo:
    forbid:
      - '^fmt\.Print(f|ln)?$'
    exclude-godoc-examples: true

  gocritic:
    enabled-tags: [diagnostic, style, performance, experimental]
    disabled-checks: [unnamedResult, whyNoLint]

  revive:
    rules:
      - name: exported
      - name: package-comments
        disabled: true
      - name: var-naming
      - name: error-return
      - name: error-strings
      - name: error-naming

  staticcheck:
    checks: ["all"]

  gosec:
    includes:
      - G101,G102,G103,G104,G106,G107,G201,G202,G203,G204,
        G301,G302,G303,G304,G305,G306,G307,G401,G402,G403,
        G404,G501,G502,G503,G504,G505,G601

issues:
  exclude-rules:
    - path: _test\.go
      linters: [gosec, errcheck]

    - linters: [gosec]
      text: "G404"
      path: _test\.go

    - path: _mock\.go
      linters: [all]

    # Allow fmt.Print* in demo binaries
    - path: ^cmd/
      linters: [forbidigo]

  max-issues-per-linter: 0
  max-same-issues: 0
  new: false

output:
  format: colored-line-number
  print-issued-lines: true
  print-linter-name: true
  sort-results: true
```

## CI alignment notes

* In your **Security** workflow you install `golangci-lint` via the action (good). Make sure you **also** run lint once **without** `dev` tag (or remove it here) so production code is covered.
* Consider failing the job on `gosec` findings (remove the `|| true`) once you burn down known issues.

If this repo is strictly for a **conference demo**, keep `nolintlint`, `gofmt.simplify`, `goimports.local-prefixes`, and **drop** `forbidigo/depguard`. For **production**, keep everything above—they catch the most common security/regression foot-guns.
