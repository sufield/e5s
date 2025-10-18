# What to keep for a smooth demo (recommended)

* **Run the actual app image** (not a sleep container + kubectl cp). It removes a whole step during a talk.
* **Mount the SPIRE socket** via `hostPath` (fine for single-node Minikube).
* **Expose a Service** on 8443 so you can `port-forward` easily.
* **TCP readiness probe** (optional but handy). It avoids “why is it not ready?” moments without having to deal with certs on probes.

# What you can drop for the demo (to reduce noise)

* Strict **seccomp/capabilities/read-only root FS**: great for prod, not required for a short demo.
* Detailed **resource requests/limits**: helpful for clusters, but not critical for Minikube.
* Extra **env overrides**: if your zero-config server auto-detects socket and uses `:8443` by default, skip them.

# Minimal demo manifest

If you want the cleanest possible file for slides/live typing:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mtls-server
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: mtls-server
  template:
    metadata:
      labels:
        app: mtls-server
    spec:
      securityContext:
        runAsNonRoot: true
      volumes:
        - name: spire-socket
          hostPath:
            path: /tmp/spire-agent/public
            type: Directory
      containers:
        - name: mtls-server
          image: ghcr.io/yourorg/zero-trust-server:latest  # your built demo image
          ports:
            - name: https
              containerPort: 8443
          volumeMounts:
            - name: spire-socket
              mountPath: /spire-socket
              readOnly: true
          # Optional but demo-friendly (doesn't need client certs)
          readinessProbe:
            tcpSocket: { port: 8443 }
            initialDelaySeconds: 2
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata:
  name: mtls-server
  namespace: default
spec:
  selector:
    app: mtls-server
  ports:
    - name: https
      port: 8443
      targetPort: 8443
```

# Demo flow

1. `kubectl apply -f mtls-server.yaml`
2. `kubectl get pods -w` (wait for **READY**)
3. `kubectl port-forward svc/mtls-server 8443:8443`
4. Call it from your mTLS client (or a demo client pod) — the zero-config server should already be using the SPIRE socket and authorize based on your built-in defaults.

