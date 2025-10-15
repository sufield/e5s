# Iteration 2: mTLS HTTP Client (Historical)

**Status**: Completed

Implemented an mTLS HTTP client (outbound adapter) that presents X.509 SVIDs to servers and verifies server identity. The client uses go-spiffe's workload API for automatic certificate rotation and supports all standard HTTP methods (GET, POST, PUT, DELETE, PATCH). Features include connection pooling, configurable timeouts, and seamless integration with the Iteration 1 server for mutual TLS authentication.

