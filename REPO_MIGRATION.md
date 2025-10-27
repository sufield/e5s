You’re doing two related moves:

1. Change repo location (from `github.com/pocket/hexagon/spire` to `github.com/sufield/e5s`)
2. Update import paths in the code so it builds under the new module name

---

Step 1. Create the new repo under your account

1. On GitHub (logged in as `sufield`), create a new repo called `e5s`.

   * You can start it empty (no README, no license, no .gitignore).
   * You can create it as **private for now**. This is created as public repo.

At this point you have:

* old repo: `github.com/pocket/hexagon/spire` (private, current code)
* new repo: `github.com/sufield/e5s` (empty)

You are not "renaming" across orgs. You are pushing the same codebase to a new remote with a new module path.

---

Step 2. Clone the old repo locally (if you don’t already have it)

```bash
git clone git@github.com:pocket/hexagon.git
cd hexagon/spire
```

If `spire` is itself the module root (has its own `go.mod`), `cd` there.
If `hexagon` is the module root and `spire` is just a subdir, you should clarify the module boundary. The new repo `e5s` should correspond to one Go module. I'll assume `spire` is (or will become) its own module.

If `spire` is not yet its own module: run `git subtree split` or copy only the `spire` subtree into a fresh working dir and `git init` there. Otherwise, continue.

---

Step 3. Update go.mod to the new module path

Open `go.mod`.

Change the first line from something like:

```go
module github.com/pocket/hexagon/spire
```

to:

```go
module github.com/sufield/e5s
```

Save.

Then run:

```bash
go mod tidy
```

This matters because all internal imports in your code should now resolve as `github.com/sufield/e5s/...`.

---

Step 4. Update all internal imports

Anywhere you currently import:

```go
"github.com/pocket/hexagon/spire/internal/..."
```

change it to:

```go
"github.com/sufield/e5s/internal/..."
```

This includes adapters, domain, ports, app, etc.

Fast way (from module root):

```bash
grep -RIl 'github.com/pocket/hexagon/spire' . \
  | xargs sed -i '' 's#github.com/pocket/hexagon/spire#github.com/sufield/e5s#g'
```

On macOS `sed -i ''`. On Linux `sed -i`.

Then run:

```bash
go vet ./...
go build ./...
go test ./...
```

Fix anything that still references the old path.

---

Step 5. Commit the rename locally

```bash
git add .
git commit -m "chore: rename module to github.com/sufield/e5s"
```

This commit includes:

* `go.mod` updated
* import paths updated

You have not touched the original `pocket` remote yet.

---

Step 6. Point this local repo at the new GitHub repo

Add the new remote:

```bash
git remote remove origin
git remote add origin git@github.com:sufield/e5s.git
```

Then push:

```bash
git push -u origin main
```

or if your default branch was `master` or something else, use that name.

At this point:

* Code now lives in `github.com/sufield/e5s`
* Import path and `go.mod` match that location
* Anyone can `go get github.com/sufield/e5s` once you make it public

---

Step 7. (Optional) Archive or restrict the old repo

In the old org/account (`pocket`):

* You can mark the old repo read-only or archive it to avoid people importing from the old module path.
* If coworkers rely on the old path, add a short README that says:
  “This code moved to github.com/sufield/e5s; update imports and go.mod.”

Do not try to “redirect” a Go module across orgs using GitHub rename — GitHub will 301 `pocket/...` → `sufield/...` only if it’s literally the same repo moved/renamed, not if you created a new repo under a different account. Since you're doing a cross-account move, consumers must update their imports.

---

Step 8. Make the new repo public (when ready)

In `github.com/sufield/e5s` repo settings:

* Flip visibility to Public
* Enable CodeQL under “Security / Code scanning” in that repo

Now CodeQL will run in CI if you enable GitHub's default CodeQL workflow or add `.github/workflows/codeql.yml`.

---

Summary of what you actually have to do in code:

1. Change `module github.com/pocket/hexagon/spire` → `module github.com/sufield/e5s` in `go.mod`.
2. Rewrite imports `github.com/pocket/hexagon/spire/...` → `github.com/sufield/e5s/...`.
3. Push that code to a new repo at `github.com/sufield/e5s`.
4. Make that repo public when you're ready to run CodeQL.

After step 3, your code is using the new identity everywhere and will build under the `sufield/e5s` module name.