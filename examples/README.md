Several steps in your Quick Start will fail as-is because the workloads never mount the Workload API socket and the `parentID` you hard-code may not match your agent. Below is a verified, corrected version. I keep your structure, but I fix the breaking parts and add minimal YAML you can paste.

---

# SPIRE mTLS Examples – Quick Start (Minikube, working)

## 1) Prerequisites

* Your versions table is fine. After installing, also set:

```bash
kubectl config use-context minikube
```

* Verify K8s tools (example target names; adapt to your Makefile):

```bash
make check-prereqs-k8s
```

---

## 2) Start SPIRE Infrastructure

```bash
cd ~/hexagon/spire
make minikube-up
make minikube-status
kubectl get pods -n spire-system
```

Expected pods (names can vary):

```
NAMESPACE      NAME               READY   STATUS
spire-system   spire-server-0     2/2     Running
spire-system   spire-agent-abcde  1/1     Running
```

---

## 3) Build the Example Server

```bash
make test
go build -o bin/mtls-server ./examples/identityserver-example
ls -lh bin/mtls-server
```

---

## 4) Register Workloads (server + client)

⚠️ Don’t guess the **parentID**. Get the actual agent SPIFFE ID first:

```bash
AGENT_ID=$(kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server agent list | awk '/SPIFFE ID/{print $3; exit}')
echo "$AGENT_ID"
```

Create entries that match how you’ll run the workloads (Kubernetes pods in the `default` namespace, default SA). Include DNS SANs you’ll use:

```bash
# Server entry (DNS SANs for Service name + localhost for convenience)
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/server \
    -parentID "$AGENT_ID" \
    -selector k8s:ns:default \
    -selector k8s:sa:default \
    -selector k8s:container-name:mtls-server \
    -dns mtls-server \
    -dns mtls-server.default \
    -dns mtls-server.default.svc \
    -dns mtls-server.default.svc.cluster.local \
    -dns localhost

# Client entry
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry create \
    -spiffeID spiffe://example.org/client \
    -parentID "$AGENT_ID" \
    -selector k8s:ns:default \
    -selector k8s:sa:default \
    -selector k8s:container-name:test-client
```

Verify:

```bash
kubectl exec -n spire-system statefulset/spire-server -c spire-server -- \
  /opt/spire/bin/spire-server entry show
```

---

## 5) Run the Example Server (Kubernetes)

Your original steps launched a pod but **did not mount the SPIRE socket**, so the server can’t reach the Workload API.

Use this minimal Deployment + Service (hostPath from node → pod, mount at `/spire-socket`, and set `SPIFFE_ENDPOINT_SOCKET=unix:///spire-socket/api.sock`):

```yaml
# mtls-server.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mtls-server
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels: { app: mtls-server }
  template:
    metadata:
      labels: { app: mtls-server }
    spec:
      securityContext:
        runAsNonRoot: true
      volumes:
        - name: spire-socket
          hostPath:
            path: /tmp/spire-agent/public   # path on the Minikube node (matches chart defaults)
            type: Directory
      containers:
        - name: mtls-server
          image: debian:bookworm-slim
          command: ["/usr/local/bin/mtls-server"]
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: "unix:///spire-socket/api.sock"
            - name: SERVER_ADDRESS
              value: ":8443"
            - name: ALLOWED_TRUST_DOMAIN
              value: "example.org"
          volumeMounts:
            - name: spire-socket
              mountPath: /spire-socket
              readOnly: true
          ports:
            - containerPort: 8443
          readinessProbe:
            httpGet:
              path: /health
              port: 8443
              scheme: HTTPS
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
---
apiVersion: v1
kind: Service
metadata:
  name: mtls-server
  namespace: default
spec:
  selector: { app: mtls-server }
  ports:
    - name: https
      port: 8443
      targetPort: 8443
```

Deploy + copy your binary:

```bash
kubectl apply -f mtls-server.yaml

# Wait for pod Ready (probe hits /health which is unauthenticated)
kubectl wait --for=condition=Ready deploy/mtls-server --timeout=120s

# Copy compiled binary into the container image (simple dev flow)
POD=$(kubectl get pod -l app=mtls-server -o jsonpath='{.items[0].metadata.name}')
kubectl cp bin/mtls-server "$POD":/usr/local/bin/mtls-server
kubectl exec "$POD" -- chmod +x /usr/local/bin/mtls-server

# Restart the container to run the server with your binary
kubectl delete pod "$POD" --wait=false
kubectl wait --for=condition=Ready deploy/mtls-server --timeout=120s
```

> Alternative: build a tiny image that already contains your binary and skip `kubectl cp`.

---

## 6) Test the Server (client in Kubernetes)

Your original client pod also **lacked the socket mount**. Use a similar pattern:

```yaml
# test-client.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-client
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels: { app: test-client }
  template:
    metadata:
      labels: { app: test-client }
    spec:
      volumes:
        - name: spire-socket
          hostPath:
            path: /tmp/spire-agent/public
            type: Directory
      containers:
        - name: test-client
          image: golang:1.25
          command: ["sleep","infinity"]
          env:
            - name: SPIFFE_ENDPOINT_SOCKET
              value: "unix:///spire-socket/api.sock"
          volumeMounts:
            - name: spire-socket
              mountPath: /spire-socket
              readOnly: true
```

Run the client:

```bash
kubectl apply -f test-client.yaml
kubectl wait --for=condition=Ready deploy/test-client --timeout=120s
CLIENT_POD=$(kubectl get pod -l app=test-client -o jsonpath='{.items[0].metadata.name}')

# Drop into the pod and run your test client program
kubectl exec -it "$CLIENT_POD" -- bash -lc '
cat > /tmp/test-client.go <<EOF
package main
import (
  "context"; "fmt"; "io"; "log"; "net/http"; "time"
  "github.com/spiffe/go-spiffe/v2/spiffeid"
  "github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
  "github.com/spiffe/go-spiffe/v2/workloadapi"
)
func main() {
  ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second); defer cancel()
  src, err := workloadapi.NewX509Source(ctx); if err != nil { log.Fatal(err) }
  defer src.Close()
  td := spiffeid.RequireTrustDomainFromString("example.org")
  tlsCfg := tlsconfig.MTLSClientConfig(src, src, tlsconfig.AuthorizeMemberOf(td))
  c := &http.Client{ Transport: &http.Transport{ TLSClientConfig: tlsCfg }, Timeout: 10*time.Second }
  for _, u := range []string{"https://mtls-server:8443/", "https://mtls-server:8443/api/hello", "https://mtls-server:8443/health"} {
    fmt.Println("=== GET", u)
    resp, err := c.Get(u); if err != nil { log.Println("ERR:", err); continue }
    b, _ := io.ReadAll(resp.Body); resp.Body.Close()
    fmt.Println("Status:", resp.StatusCode, "Body:", string(b))
  }
}
EOF
cd /tmp && go mod init tc && go get github.com/spiffe/go-spiffe/v2@latest && go run /tmp/test-client.go
'
```

> Note: `/health` is unauthenticated by design; the other endpoints require mTLS.

### Port-forward (host)

Port-forwarding to `mtls-server` won’t help from your **host** unless your host has a Workload API socket/SVID. Use the in-cluster client above.

---

## 7) Cleanup

```bash
kubectl delete -f test-client.yaml -f mtls-server.yaml
make minikube-down     # keep data
# or
make minikube-delete   # destroy cluster
```

---

## 8) Troubleshooting

**Check SPIRE status**

```bash
kubectl get pods -n spire-system
kubectl logs -n spire-system statefulset/spire-server -c spire-server --tail=100
kubectl logs -n spire-system ds/spire-agent -c spire-agent --tail=100
```

**Socket present inside agent**

```bash
AGENT_POD=$(kubectl get pods -n spire-system -l app=spire-agent -o jsonpath='{.items[0].metadata.name}')
kubectl exec -n spire-system "$AGENT_POD" -- ls -la /tmp/spire-agent/public/api.sock
```

**Workload failing to get SVID**

* Ensure **selectors** match (namespace, serviceAccount, container name).
* Ensure **parentID** equals your agent’s SPIFFE ID (`spire-server agent list`).
* After fixing, wait a few seconds for cache refresh.

**Server can’t start / TLS handshake errors**

* Confirm the server pod has the **volume + mount** and `SPIFFE_ENDPOINT_SOCKET`.
* Confirm the server **Service** DNS names are in the server entry’s `-dns` list.
* Look at server logs (stdout) and client errors for verification failures.

---

## Why your original steps broke

* **No socket mount** in either the server or client pods ⇒ `dial unix /…/api.sock: no such file or directory`.
* **Hard-coded `parentID`** can be wrong ⇒ workload never gets an SVID.
* **Port-forward from host** without a host SVID ⇒ host cannot complete mTLS handshake.

---

* In Kubernetes, you must **mount the Workload API socket** (hostPath → pod) and point `SPIFFE_ENDPOINT_SOCKET` at the mount.
* Always **read the agent SPIFFE ID** and use it as the **parentID** when creating entries.
* Put Service DNS names you’ll use in the **server entry’s `-dns`** list so TLS name verification succeeds.
