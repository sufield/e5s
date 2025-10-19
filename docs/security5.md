Looks solid for a hands-on demo. It will work, but a few small tweaks will save you from common “on-stage” gotchas and keep it consistent with the server manifest you shared earlier.

## What’s good

* Mounts the SPIRE public socket dir and points `SPIFFE_ENDPOINT_SOCKET` at `unix:///spire-socket/api.sock`.
* Runs non-root and drops all caps.
* Uses the official `golang` image so you can compile/run a quick client in-pod.

## Recommended tweaks (dev-friendly, not production-hardening)

1. **Use Debian’s standard nobody UID/GID (65534)** for consistency with the server pod and fewer permission surprises.
2. **Disable SA token auto-mount** (you don’t call the K8s API):

   ```yaml
   automountServiceAccountToken: false
   ```
3. **Add a seccomp profile** (free safety, no downside for demos):

   ```yaml
   securityContext:
     seccompProfile:
       type: RuntimeDefault
   ```
4. **Socket access reliability**: keep `fsGroup` to ensure the pod can read the hostPath socket; you already set it—good.
5. **Tooling inside the client pod** (optional but handy): the `golang` image is slim; if you plan to `go run` with private repos or curl endpoints, install tools ad-hoc:

   ```bash
   apt-get update && apt-get install -y ca-certificates curl git && update-ca-certificates
   ```
6. **Resources**: compiling in-pod can spike CPU; if you see slow builds, bump to `cpu: 300m`.

## Tiny patch (inline)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-client
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-client
  template:
    metadata:
      labels:
        app: test-client
    spec:
      automountServiceAccountToken: false
      securityContext:
        runAsNonRoot: true
        runAsUser: 65534        # Debian 'nobody'
        fsGroup: 65534
        seccompProfile:
          type: RuntimeDefault
      volumes:
        - name: spire-socket
          hostPath:
            # Matches the SPIRE chart default public socket directory.
            path: /tmp/spire-agent/public
            type: Directory
      containers:
        - name: test-client
          image: golang:1.22-bookworm     # (or your chosen Go version tag)
          command: ["sleep","infinity"]
          securityContext:
            allowPrivilegeEscalation: false
            runAsNonRoot: true
            runAsUser: 65534
            capabilities: { drop: ["ALL"] }
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: "unix:///spire-socket/api.sock"
          volumeMounts:
            - name: spire-socket
              mountPath: /spire-socket
              readOnly: true
          resources:
            requests:
              memory: "128Mi"
              cpu: "100m"
            limits:
              memory: "256Mi"
              cpu: "300m"        # small bump helps with 'go run'
```

### Quick usage recap (with conveniences)

```bash
# Deploy and wait
kubectl apply -f examples/test-client.yaml
kubectl wait --for=condition=Available deploy/test-client --timeout=60s

# Exec into pod
POD=$(kubectl get pod -l app=test-client -o jsonpath='{.items[0].metadata.name}')
kubectl exec -it "$POD" -- bash

# (inside the pod) optional tools for demos:
apt-get update && apt-get install -y ca-certificates curl git && update-ca-certificates

# (inside the pod) minimal Go client example:
cat <<'EOF' > /tmp/client.go
package main

import (
  "context"
  "crypto/tls"
  "fmt"
  "io"
  "net/http"
  "time"
  "github.com/spiffe/go-spiffe/v2/spiffeid"
  "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
  "github.com/spiffe/go-spiffe/v2/workloadapi"
)

func main() {
  ctx := context.Background()
  src, _ := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(
    workloadapi.WithAddr("unix:///spire-socket/api.sock"),
  ))
  defer src.Close()
  serverID := spiffeid.RequireFromString("spiffe://example.org/server")
  tlsCfg := tlsconfig.MTLSClientConfig(src, src, tlsconfig.AuthorizeID(serverID))
  tlsCfg.MinVersion = tls.VersionTLS13
  c := &http.Client{ Transport: &http.Transport{ TLSClientConfig: tlsCfg }, Timeout: 10*time.Second }
  resp, err := c.Get("https://mtls-server:8443/api/hello")
  if err != nil { panic(err) }
  defer resp.Body.Close()
  b, _ := io.ReadAll(resp.Body)
  fmt.Println(string(b))
}
EOF
go run /tmp/client.go
```

That’s it—keeps the demo flow simple, avoids permission snags, and gives you a couple of escape hatches (curl/git) if you need to debug live.
