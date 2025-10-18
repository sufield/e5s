# Dockerfile for zero-config mTLS server demo
#
# Build:
#   docker build -t your-registry/mtls-server:latest -f examples/zeroconfig-example/Dockerfile .
#
# For multi-arch builds:
#   docker buildx build --platform linux/amd64,linux/arm64 -t your-registry/mtls-server:latest .
#
# Push:
#   docker push your-registry/mtls-server:latest

FROM golang:1.25.1-bookworm AS builder

WORKDIR /workspace

# Copy go.mod and go.sum first for better caching
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /mtls-server ./examples/zeroconfig-example

# Production image
FROM gcr.io/distroless/static-debian12:nonroot

# Copy the binary from builder
COPY --from=builder /mtls-server /usr/local/bin/mtls-server

# distroless/static runs as nonroot user by default (uid 65532)
USER nonroot:nonroot

# Server listens on 8443 by default
EXPOSE 8443

ENTRYPOINT ["/usr/local/bin/mtls-server"]
