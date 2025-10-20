Short answer: **you don’t need two “Identity”s. Keep a single source of truth** and (optionally) re-export it from your public facade.

Right now you have:

* **internal/ports/identity.go** — defines the canonical `ports.Identity` type + context helpers. This is perfect as the **contract** adapters use.
* **pkg/zerotrustserver/identity.go** — defines a second function that returns `ports.Identity`. This creates duplication noise (two places to look), but it’s fine to keep a *facade* if it doesn’t redefine the type.

### Recommended cleanup

1. **Do not duplicate the type.** Keep `Identity` defined only in `internal/ports`.
2. In the public facade (`pkg/zerotrustserver`), **type-alias** to avoid import leaks and provide a single public entry point.

```go
// pkg/zerotrustserver/identity.go
package zerotrustserver

import (
    "context"

    "github.com/pocket/hexagon/spire/internal/ports"
)

// Re-export the type so users only import the facade package.
type Identity = ports.Identity

// Public accessor for handlers to read the authenticated identity.
// This keeps WithIdentity internal so users cannot forge identities.
func PeerIdentity(ctx context.Context) (Identity, bool) {
    return ports.PeerIdentity(ctx)
}
```

3. **Keep `WithIdentity` unexported to users** (only adapters call it). That preserves the security boundary: apps can *read* identity, not *inject* it.

### Why this fits hexagonal architecture

* **One contract** (`ports.Identity`) = clear boundary between adapters and app code.
* **Facade re-export** = a clean public API (`zerotrustserver.PeerIdentity(ctx)`) for developers, without exposing `internal/ports` directly.
* **No duplication** = less drift, fewer docs to maintain.

### What to delete/change

* Remove the separate `Identity()` func in `pkg/zerotrustserver` that duplicates behavior. Replace it with the alias + `PeerIdentity` above.
* Ensure all examples import only `pkg/zerotrustserver` for handler code:

  ```go
  id, ok := zerotrustserver.PeerIdentity(r.Context())
  ```

This gives you a single Identity definition, a tidy public API, and keeps your hexagonal boundaries intact.
