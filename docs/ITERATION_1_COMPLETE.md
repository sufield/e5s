# Iteration 1: mTLS HTTP Server (Historical)

**Status**: Completed

Implemented an mTLS HTTP server (inbound adapter) that authenticates clients using X.509 SVIDs. The server uses go-spiffe's workload API for automatic certificate rotation and provides middleware to extract client SPIFFE IDs from TLS connections. Handler functions receive authenticated client identity via request context, enabling application-level authorization decisions. The implementation supports all standard go-spiffe authorizers and includes graceful shutdown with proper resource cleanup.
