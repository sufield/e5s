# Dockerfile for zero-config mTLS server demo
# Optimized for Falco runtime security monitoring
#
# Build:
#   docker build -t your-registry/mtls-server:latest -f examples/zeroconfig-example/Dockerfile .
#
# For multi-arch builds:
#   docker buildx build --platform linux/amd64,linux/arm64 -t your-registry/mtls-server:latest .
#
# Push:
#   docker push your-registry/mtls-server:latest
#
# Security Features:
#   - Distroless base (minimal attack surface)
#   - Non-root user (UID 65532)
#   - Static binary (no dependencies)
#   - Security labels for Falco monitoring
#   - Reproducible builds with version metadata

FROM golang:1.25.1-bookworm AS builder

WORKDIR /workspace

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary with security hardening flags
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a \
    -trimpath \
    -ldflags="-s -w -X main.version=$(git describe --tags --always --dirty 2>/dev/null || echo 'dev') -X main.buildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    -o /mtls-server \
    ./examples/zeroconfig-example

# Production image - Distroless for minimal attack surface
FROM gcr.io/distroless/static-debian12:nonroot

# OCI image labels for metadata and documentation
LABEL org.opencontainers.image.title="SPIRE mTLS Server" \
      org.opencontainers.image.description="Zero-config mTLS server using SPIFFE/SPIRE for authentication" \
      org.opencontainers.image.source="https://github.com/pocket/hexagon/spire" \
      org.opencontainers.image.vendor="Pocket Network" \
      org.opencontainers.image.licenses="Apache-2.0"

# Security labels for Falco monitoring and policy enforcement
LABEL security.falco.rules="spire-mtls" \
      security.capabilities.drop="ALL" \
      security.seccomp.profile="runtime/default" \
      security.readonly.rootfs="false" \
      security.user.uid="65532" \
      security.user.name="nonroot"

# Copy the binary from builder
COPY --from=builder /mtls-server /usr/local/bin/mtls-server

# distroless/static runs as nonroot user by default (uid 65532)
# This matches our Kubernetes securityContext
USER nonroot:nonroot

# Default environment variables (can be overridden in K8s)
ENV SERVER_ADDRESS=":8443" \
    SPIFFE_ENDPOINT_SOCKET="unix:///spire-socket/api.sock"

# Server listens on 8443 by default (mTLS)
EXPOSE 8443

# Entrypoint - using ENTRYPOINT for better signal handling
ENTRYPOINT ["/usr/local/bin/mtls-server"]

# Expected volume mounts (documentation):
# - /spire-socket: SPIRE agent socket directory (read-only)
#
# Expected Kubernetes securityContext:
# - runAsNonRoot: true
# - runAsUser: 65532
# - readOnlyRootFilesystem: false (Go needs /tmp for runtime)
# - allowPrivilegeEscalation: false
# - capabilities: drop ALL
# - seccompProfile: RuntimeDefault
