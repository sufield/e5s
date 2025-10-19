The job is failing because your codebase and several dependencies require Go 1.24 or Go 1.25, but your workflow runner is using Go 1.23. Example errors:

- package requires newer Go version go1.25 (application built with go1.23)
- package requires newer Go version go1.24 (application built with go1.23)

**Solution:**  
Update the Go version in your workflow file (`.github/workflows/security.yml`) to at least Go 1.25.

**Code suggestion:**  
Find the `setup-go` or `actions/setup-go` step in your workflow, and change it to:

```yaml
- name: Set up Go
  uses: actions/setup-go@v4
  with:
    go-version: '1.25'
```

If you use a matrix, ensure `go-version: ['1.25']` is present.

**Next step:**  
Commit this change and re-run the workflow. This will resolve the version mismatch errors and allow your job to pass.

GitHub Copilot Chat Assistant — Root cause
- The workflow sets up Go 1.23 (actions/setup-go go-version: '1.23') but your module requires go 1.25.1 (go.mod: go 1.25.1). govulncheck and other tools try to build packages and fail with "package requires newer Go version go1.24 / go1.25 (application built with go1.23)".

Fix (minimal, high-confidence)
1) Update .github/workflows/security.yml to install a Go version that matches go.mod (1.25.1).
2) Avoid reusing an old cache built with Go 1.23 by including the Go version in the cache key.

Replace the "Set up Go" and "Cache Go build" steps with the following (use this exact file in your repo at ref d695a3104ffdc61612e1e4f19885aa7b40bb418b):
https://github.com/bparanj/atum/blob/d695a3104ffdc61612e1e4f19885aa7b40bb418b/.github/workflows/security.yml

Suggested snippet to apply (replace lines 25–40 in that file):

```yaml
- name: Set up Go
  uses: actions/setup-go@v5
  with:
    go-version: '1.25.1'
    cache: true

- name: Cache Go build
  uses: actions/cache@v4
  with:
    path: |
      ~/.cache/go-build
      ~/go/pkg/mod
    # include Go version in the cache key to avoid restoring artifacts built with an older toolchain
    key: ${{ runner.os }}-go-1.25.1-${{ hashFiles('**/go.sum') }}
    restore-keys: |
      ${{ runner.os }}-go-1.25.1-
```

Notes and optional hardening
- Using the exact patch version (1.25.1) matches your go.mod and is recommended to prevent subtle mismatches. You can use '1.25' if you prefer to track minor updates.
- If you want a dynamic/cache-proof approach, set an env var GO_VERSION and refer to it in the key; or include ${{ github.runner.os }}-${{ steps.setup-go.outputs.go-version }} if you capture setup-go output.
- If you see residual cache problems after this change, manually clear the workflow cache (or bump the key) and re-run the job.

Why this will fix it
- go tool and many modules are built/checked with the runner's Go toolchain. Using a toolchain older than the module or dependency constraints produces the "package requires newer Go version" errors seen in the logs. Upgrading the runner to 1.25.1 (and preventing restoration of older caches) resolves those build-time version mismatches.

After you commit the change, re-run the failed workflow.