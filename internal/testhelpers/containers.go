// Package testhelpers provides containerized SPIRE infrastructure for integration testing.
//
// This approach uses testcontainers-go to run SPIRE server and agent in Docker containers,
// providing a more portable testing environment that doesn't require local SPIRE binaries.
package testhelpers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// SPIREContainers provides a containerized SPIRE test environment.
//
// This sets up:
//   - SPIRE server container
//   - SPIRE agent container
//   - Shared volume for agent socket
//   - Workload API socket exposed via bind mount
//
// Example usage:
//
//	func TestWithContainerizedSPIRE(t *testing.T) {
//	    spire, cleanup := testhelpers.SetupSPIREContainers(t)
//	    defer cleanup()
//
//	    // Use spire.SocketPath for workload API connection
//	    source, err := e5s.NewIdentitySource(context.Background(), e5s.Config{
//	        WorkloadSocket: spire.SocketPath,
//	    })
//	    // ... test code ...
//	}
type SPIREContainers struct {
	t *testing.T

	// SocketPath is the path to the Workload API Unix domain socket.
	// This is mounted from the agent container to the host filesystem.
	SocketPath string

	// TrustDomain is the SPIFFE trust domain for this test instance.
	TrustDomain string

	// ServerContainer is the running SPIRE server container.
	ServerContainer testcontainers.Container

	// AgentContainer is the running SPIRE agent container.
	AgentContainer testcontainers.Container

	// ServerHost is the hostname/IP for connecting to the server.
	ServerHost string

	// ServerPort is the port the SPIRE server is listening on.
	ServerPort int

	// Network is the Docker network connecting server and agent.
	Network *testcontainers.DockerNetwork
}

// SetupSPIREContainers creates and starts containerized SPIRE infrastructure for testing.
//
// This function:
//  1. Creates a Docker network for SPIRE components
//  2. Starts a SPIRE server container
//  3. Generates a join token
//  4. Starts a SPIRE agent container with the join token
//  5. Waits for both components to become healthy
//  6. Creates a workload registration entry for tests
//
// Requirements:
//   - Docker daemon running and accessible
//   - Docker images: ghcr.io/spiffe/spire-server:1.11, ghcr.io/spiffe/spire-agent:1.11
//
// The containers are automatically cleaned up when the test completes via t.Cleanup().
//
// Skip test if Docker is not available:
//
//	func TestMyFeature(t *testing.T) {
//	    if testing.Short() {
//	        t.Skip("Skipping container-based test in short mode")
//	    }
//	    spire, cleanup := testhelpers.SetupSPIREContainers(t)
//	    defer cleanup()
//	    // ... test code ...
//	}
func SetupSPIREContainers(t *testing.T) (*SPIREContainers, func()) {
	t.Helper()

	ctx := context.Background()
	trustDomain := "example.org"

	// Create Docker network for SPIRE components
	networkReq := testcontainers.NetworkRequest{
		Name:           fmt.Sprintf("spire-test-%d", time.Now().UnixNano()),
		CheckDuplicate: true,
	}

	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: networkReq,
	})
	if err != nil {
		t.Fatalf("Failed to create Docker network: %v", err)
	}

	dockerNetwork := network.(*testcontainers.DockerNetwork)

	sc := &SPIREContainers{
		t:           t,
		TrustDomain: trustDomain,
		Network:     dockerNetwork,
	}

	// Start SPIRE server
	sc.startServerContainer(ctx)

	// Generate join token
	joinToken := sc.generateJoinToken(ctx)

	// Start SPIRE agent
	sc.startAgentContainer(ctx, joinToken)

	// Create workload entry
	sc.createContainerWorkloadEntry(ctx)

	// Cleanup function
	cleanup := func() {
		cleanupCtx := context.Background()

		if sc.AgentContainer != nil {
			t.Log("Terminating SPIRE agent container...")
			if err := sc.AgentContainer.Terminate(cleanupCtx); err != nil {
				t.Logf("Warning: failed to terminate agent container: %v", err)
			}
		}

		if sc.ServerContainer != nil {
			t.Log("Terminating SPIRE server container...")
			if err := sc.ServerContainer.Terminate(cleanupCtx); err != nil {
				t.Logf("Warning: failed to terminate server container: %v", err)
			}
		}

		if sc.Network != nil {
			t.Log("Removing Docker network...")
			if err := sc.Network.Remove(cleanupCtx); err != nil {
				t.Logf("Warning: failed to remove network: %v", err)
			}
		}
	}

	t.Cleanup(cleanup)

	return sc, cleanup
}

// startServerContainer starts the SPIRE server container.
func (sc *SPIREContainers) startServerContainer(ctx context.Context) {
	sc.t.Helper()

	// Server configuration
	serverConfig := `
server {
    bind_address = "0.0.0.0"
    bind_port = "8081"
    trust_domain = "` + sc.TrustDomain + `"
    data_dir = "/opt/spire/data/server"
    log_level = "DEBUG"
}

plugins {
    DataStore "sql" {
        plugin_data {
            database_type = "sqlite3"
            connection_string = "/opt/spire/data/server/datastore.sqlite3"
        }
    }

    KeyManager "memory" {
        plugin_data {}
    }

    NodeAttestor "join_token" {
        plugin_data {}
    }
}
`

	req := testcontainers.ContainerRequest{
		Image:        "ghcr.io/spiffe/spire-server:1.11",
		ExposedPorts: []string{"8081/tcp"},
		Networks:     []string{sc.Network.Name},
		NetworkAliases: map[string][]string{
			sc.Network.Name: {"spire-server"},
		},
		WaitingFor: wait.ForLog("Starting Server APIs").WithStartupTimeout(60 * time.Second),
		Cmd:        []string{"-config", "/opt/spire/conf/server/server.conf"},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "", // Will use Reader instead
				ContainerFilePath: "/opt/spire/conf/server/server.conf",
				FileMode:          0600,
				Reader:            nil, // Set below
			},
		},
	}

	// Add config file
	req.Files[0].Reader = bytes.NewReader([]byte(serverConfig))

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		sc.t.Fatalf("Failed to start SPIRE server container: %v", err)
	}

	sc.ServerContainer = container

	// Get server host and port
	host, err := container.Host(ctx)
	if err != nil {
		sc.t.Fatalf("Failed to get server host: %v", err)
	}

	port, err := container.MappedPort(ctx, "8081")
	if err != nil {
		sc.t.Fatalf("Failed to get server port: %v", err)
	}

	sc.ServerHost = host
	sc.ServerPort = port.Int()

	sc.t.Logf("SPIRE server started: %s:%d", sc.ServerHost, sc.ServerPort)
}

// generateJoinToken generates a join token for the agent.
func (sc *SPIREContainers) generateJoinToken(ctx context.Context) string {
	sc.t.Helper()

	agentID := fmt.Sprintf("spiffe://%s/test-agent", sc.TrustDomain)

	exitCode, reader, err := sc.ServerContainer.Exec(ctx, []string{
		"/opt/spire/bin/spire-server",
		"token", "generate",
		"-spiffeID", agentID,
	})

	if err != nil {
		sc.t.Fatalf("Failed to execute token generate: %v", err)
	}

	// Read output
	outputBytes, err := io.ReadAll(reader)
	if err != nil {
		sc.t.Fatalf("Failed to read token output: %v", err)
	}

	if exitCode != 0 {
		sc.t.Fatalf("Failed to generate join token: exit=%d, output=%s", exitCode, string(outputBytes))
	}

	// Parse token from output
	outputStr := string(outputBytes)
	token, err := parseToken(outputStr)
	if err != nil {
		sc.t.Fatalf("Failed to parse join token: %v\nOutput: %s", err, outputStr)
	}

	sc.t.Logf("Generated join token for agent")
	return token
}

// startAgentContainer starts the SPIRE agent container.
func (sc *SPIREContainers) startAgentContainer(ctx context.Context, joinToken string) {
	sc.t.Helper()

	// Agent configuration
	agentConfig := `
agent {
    data_dir = "/opt/spire/data/agent"
    log_level = "DEBUG"
    trust_domain = "` + sc.TrustDomain + `"
    server_address = "spire-server"
    server_port = "8081"
    insecure_bootstrap = true
}

plugins {
    KeyManager "memory" {
        plugin_data {}
    }

    NodeAttestor "join_token" {
        plugin_data {}
    }

    WorkloadAttestor "unix" {
        plugin_data {}
    }
}
`

	// Create temp directory for socket
	socketDir := filepath.Join(sc.t.TempDir(), "spire-agent")

	req := testcontainers.ContainerRequest{
		Image:    "ghcr.io/spiffe/spire-agent:1.11",
		Networks: []string{sc.Network.Name},
		NetworkAliases: map[string][]string{
			sc.Network.Name: {"spire-agent"},
		},
		WaitingFor: wait.ForLog("Starting Workload API").WithStartupTimeout(60 * time.Second),
		Cmd: []string{
			"-config", "/opt/spire/conf/agent/agent.conf",
			"-joinToken", joinToken,
		},
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      "",
				ContainerFilePath: "/opt/spire/conf/agent/agent.conf",
				FileMode:          0600,
				Reader:            bytes.NewReader([]byte(agentConfig)),
			},
		},
		Mounts: testcontainers.Mounts(
			testcontainers.BindMount(socketDir, "/tmp/spire-agent/public"),
		),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		sc.t.Fatalf("Failed to start SPIRE agent container: %v", err)
	}

	sc.AgentContainer = container
	sc.SocketPath = filepath.Join(socketDir, "api.sock")

	sc.t.Logf("SPIRE agent started, socket: %s", sc.SocketPath)

	// Wait for socket to appear
	sc.waitForSocket(ctx)
}

// waitForSocket waits for the agent socket to become available.
func (sc *SPIREContainers) waitForSocket(ctx context.Context) {
	sc.t.Helper()

	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			sc.t.Fatal("Context cancelled while waiting for socket")
		case <-ticker.C:
			// Check if socket file exists on host
			if _, err := os.Stat(sc.SocketPath); err == nil {
				sc.t.Log("SPIRE agent socket is ready")
				return
			}
		}
	}

	sc.t.Fatalf("Timeout waiting for agent socket: %s", sc.SocketPath)
}

// createContainerWorkloadEntry creates a workload registration entry.
func (sc *SPIREContainers) createContainerWorkloadEntry(ctx context.Context) {
	sc.t.Helper()

	spiffeID := fmt.Sprintf("spiffe://%s/test-workload", sc.TrustDomain)
	parentID := fmt.Sprintf("spiffe://%s/test-agent", sc.TrustDomain)

	exitCode, reader, err := sc.ServerContainer.Exec(ctx, []string{
		"/opt/spire/bin/spire-server",
		"entry", "create",
		"-spiffeID", spiffeID,
		"-parentID", parentID,
		"-selector", "unix:uid:0", // Root in container
	})

	if err != nil {
		sc.t.Logf("Warning: Failed to execute entry create: %v", err)
		return
	}

	outputBytes, _ := io.ReadAll(reader)

	if exitCode != 0 {
		sc.t.Logf("Warning: Failed to create workload entry: exit=%d, output=%s", exitCode, string(outputBytes))
	} else {
		sc.t.Logf("Created workload entry: %s", spiffeID)
	}
}

// parseToken extracts the join token from spire-server output.
func parseToken(output string) (string, error) {
	// Output format: "Token: <token>"
	lines := []rune(output)
	var token string

	// Simple parser - look for "Token: " prefix
	tokenPrefix := "Token: "
	idx := 0
	for idx < len(lines) {
		// Find start of line
		start := idx
		// Find end of line
		end := idx
		for end < len(lines) && lines[end] != '\n' {
			end++
		}

		line := string(lines[start:end])
		if len(line) >= len(tokenPrefix) && line[:len(tokenPrefix)] == tokenPrefix {
			token = line[len(tokenPrefix):]
			break
		}

		idx = end + 1
	}

	if token == "" {
		return "", fmt.Errorf("token not found in output")
	}

	return token, nil
}
