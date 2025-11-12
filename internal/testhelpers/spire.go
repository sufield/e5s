// Package testhelpers provides test utilities for integration testing with SPIRE.
//
// This package helps set up and tear down SPIRE infrastructure for integration tests
// that require real SPIRE server and agent instances.
package testhelpers

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// SPIRETest provides a test environment with running SPIRE server and agent.
//
// This sets up:
//   - Temporary directory for SPIRE data
//   - SPIRE server running in background
//   - SPIRE agent running in background
//   - Workload API socket for tests to use
//
// Example usage:
//
//	func TestWithSPIRE(t *testing.T) {
//	    spire := testhelpers.SetupSPIRE(t)
//	    defer spire.Cleanup()
//
//	    // Use spire.SocketPath for workload API connection
//	    source, err := spire.NewIdentitySource(context.Background(), spire.Config{})
//	    // ... test code ...
//	}
type SPIRETest struct {
	t *testing.T

	// SocketPath is the path to the Workload API Unix domain socket.
	// Use this to configure clients to connect to the test SPIRE agent.
	SocketPath string

	// TempDir is the temporary directory used for SPIRE data.
	// Automatically cleaned up on test completion.
	TempDir string

	// TrustDomain is the SPIFFE trust domain for this test instance.
	TrustDomain string

	// ServerPID is the PID of the SPIRE server process.
	serverPID int

	// AgentPID is the PID of the SPIRE agent process.
	agentPID int

	// serverPort is the port the SPIRE server is listening on.
	serverPort int

	// adminAPISocket is the path to the SPIRE server admin API socket.
	adminAPISocket string

	// ServerCmd is the server process handle (for cleanup).
	serverCmd *exec.Cmd

	// AgentCmd is the agent process handle (for cleanup).
	agentCmd *exec.Cmd
}

// SetupSPIRE creates and starts a SPIRE server and agent for integration testing.
//
// This function:
//  1. Creates a temporary directory for SPIRE data
//  2. Generates minimal SPIRE server and agent configurations
//  3. Starts the SPIRE server in the background
//  4. Starts the SPIRE agent in the background
//  5. Creates a workload registration entry for the test
//  6. Waits for the agent to become healthy
//
// The SPIRE infrastructure is automatically torn down when the test completes
// via t.Cleanup(), but you can also call Cleanup() explicitly if needed.
//
// Requirements:
//   - spire-server binary must be in PATH or SPIRE_SERVER env var
//   - spire-agent binary must be in PATH or SPIRE_AGENT env var
//
// Skip test if SPIRE binaries are not available:
//
//	if !testhelpers.SPIREAvailable() {
//	    t.Skip("SPIRE binaries not available")
//	}
func SetupSPIRE(t *testing.T) *SPIRETest {
	t.Helper()

	// Check if SPIRE binaries are available
	if !SPIREAvailable() {
		t.Skip("SPIRE binaries not available (set SPIRE_SERVER and SPIRE_AGENT env vars or add to PATH)")
	}

	// Create temporary directory for SPIRE data
	tempDir, err := os.MkdirTemp("", "spire-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	// Clean up temp dir on test completion
	t.Cleanup(func() {
		os.RemoveAll(tempDir)
	})

	trustDomain := "example.org"
	socketPath := filepath.Join(tempDir, "agent.sock")

	st := &SPIRETest{
		t:           t,
		SocketPath:  socketPath,
		TempDir:     tempDir,
		TrustDomain: trustDomain,
	}

	// Start SPIRE server
	st.startServer()

	// Start SPIRE agent
	st.startAgent()

	// Wait for agent to be ready
	st.waitForAgent()

	// Create workload registration for tests
	st.createWorkloadEntry()

	return st
}

// SPIREAvailable checks if SPIRE binaries are available for testing.
//
// Returns true if both spire-server and spire-agent binaries can be found
// in PATH or via SPIRE_SERVER/SPIRE_AGENT environment variables.
func SPIREAvailable() bool {
	serverBin := os.Getenv("SPIRE_SERVER")
	if serverBin == "" {
		serverBin = "spire-server"
	}

	agentBin := os.Getenv("SPIRE_AGENT")
	if agentBin == "" {
		agentBin = "spire-agent"
	}

	_, serverErr := exec.LookPath(serverBin)
	_, agentErr := exec.LookPath(agentBin)

	return serverErr == nil && agentErr == nil
}

// getFreePort finds an available TCP port on localhost.
func getFreePort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port, nil
}

// startServer starts the SPIRE server in the background.
func (st *SPIRETest) startServer() {
	st.t.Helper()

	serverDir := filepath.Join(st.TempDir, "server")
	if err := os.MkdirAll(serverDir, 0o750); err != nil {
		st.t.Fatalf("Failed to create server dir: %v", err)
	}

	// Find an available port for the SPIRE server
	serverPort, err := getFreePort()
	if err != nil {
		st.t.Fatalf("Failed to find free port: %v", err)
	}

	// Write minimal server configuration
	serverConfPath := filepath.Join(serverDir, "server.conf")
	apiSockPath := filepath.Join(serverDir, "data", "private", "api.sock")
	serverConf := fmt.Sprintf(`server {
    bind_address = "127.0.0.1"
    bind_port = "%d"
    trust_domain = "%s"
    data_dir = "%s/data"
    log_level = "DEBUG"
    socket_path = "%s"
}

plugins {
    DataStore "sql" {
        plugin_data {
            database_type = "sqlite3"
            connection_string = "%s/data/datastore.sqlite3"
        }
    }

    KeyManager "memory" {
        plugin_data {}
    }

    NodeAttestor "join_token" {
        plugin_data {}
    }
}
`, serverPort, st.TrustDomain, serverDir, apiSockPath, serverDir)

	// Store server port and admin API socket for agent and entry creation
	st.serverPort = serverPort
	st.adminAPISocket = apiSockPath

	if err := os.WriteFile(serverConfPath, []byte(serverConf), 0o600); err != nil {
		st.t.Fatalf("Failed to write server config: %v", err)
	}

	// Start server
	serverBin := os.Getenv("SPIRE_SERVER")
	if serverBin == "" {
		serverBin = "spire-server"
	}

	st.serverCmd = exec.Command(serverBin, "run", "-config", serverConfPath)
	st.serverCmd.Stdout = os.Stdout
	st.serverCmd.Stderr = os.Stderr

	if err := st.serverCmd.Start(); err != nil {
		st.t.Fatalf("Failed to start SPIRE server: %v", err)
	}

	st.serverPID = st.serverCmd.Process.Pid
	st.t.Logf("Started SPIRE server (PID %d)", st.serverPID)

	// Clean up server on test completion
	st.t.Cleanup(func() {
		if st.serverCmd != nil && st.serverCmd.Process != nil {
			st.t.Logf("Stopping SPIRE server (PID %d)", st.serverPID)
			if err := st.serverCmd.Process.Kill(); err != nil {
				st.t.Logf("Warning: failed to kill server process: %v", err)
			}
			if err := st.serverCmd.Wait(); err != nil {
				st.t.Logf("Warning: server process wait returned: %v", err)
			}
		}
	})

	// Give server time to start
	time.Sleep(2 * time.Second)
}

// startAgent starts the SPIRE agent in the background.
func (st *SPIRETest) startAgent() {
	st.t.Helper()

	agentDir := filepath.Join(st.TempDir, "agent")
	if err := os.MkdirAll(agentDir, 0o750); err != nil {
		st.t.Fatalf("Failed to create agent dir: %v", err)
	}

	// Write minimal agent configuration
	agentConfPath := filepath.Join(agentDir, "agent.conf")
	agentConf := fmt.Sprintf(`agent {
    data_dir = "%s/data"
    log_level = "DEBUG"
    trust_domain = "%s"
    server_address = "127.0.0.1"
    server_port = "%d"
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
`, agentDir, st.TrustDomain, st.serverPort)

	if err := os.WriteFile(agentConfPath, []byte(agentConf), 0o600); err != nil {
		st.t.Fatalf("Failed to write agent config: %v", err)
	}

	// Wait for the server private API socket to appear (avoid race between server start and token CLI)
	// The socket is at <data_dir>/private/api.sock, where data_dir is <tempDir>/server/data
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	tick := time.NewTicker(200 * time.Millisecond)
	defer tick.Stop()

	logTicker := time.NewTicker(5 * time.Second)
	defer logTicker.Stop()

	for {
		if _, err := os.Stat(st.adminAPISocket); err == nil {
			st.t.Log("Server API socket appeared")
			break
		}
		select {
		case <-ctx.Done():
			// Check if server process is still running
			if st.serverCmd != nil && st.serverCmd.Process != nil {
				if err := st.serverCmd.Process.Signal(os.Signal(nil)); err != nil {
					st.t.Fatalf("Server API socket did not appear: %s (server process has exited)", st.adminAPISocket)
				} else {
					st.t.Fatalf("Server API socket did not appear: %s (server process still running but no socket)", st.adminAPISocket)
				}
			} else {
				st.t.Fatalf("Server API socket did not appear: %s (server process not available)", st.adminAPISocket)
			}
		case <-logTicker.C:
			// Periodic progress logging
			st.t.Logf("Still waiting for server API socket: %s", st.adminAPISocket)
		case <-tick.C:
			// loop until socket appears or timeout
		}
	}

	// Generate join token from server using the actual private API socket
	serverBin := os.Getenv("SPIRE_SERVER")
	if serverBin == "" {
		serverBin = "spire-server"
	}

	agentID := fmt.Sprintf("spiffe://%s/test-agent", st.TrustDomain)
	tokenCmd := exec.Command(serverBin, "token", "generate",
		"-spiffeID", agentID,
		"-socketPath", st.adminAPISocket)

	tokenOutput, err := tokenCmd.CombinedOutput()
	if err != nil {
		st.t.Fatalf("Failed to generate join token: %v\nOutput: %s", err, tokenOutput)
	}

	// Parse token from output (format: "Token: <token>")
	var joinToken string
	for _, line := range strings.Split(string(tokenOutput), "\n") {
		if strings.HasPrefix(line, "Token:") {
			joinToken = strings.TrimSpace(strings.TrimPrefix(line, "Token:"))
			break
		}
	}
	if joinToken == "" {
		st.t.Fatalf("Failed to parse join token from output: %s", tokenOutput)
	}

	st.t.Logf("Generated join token for agent")

	// Start agent with join token
	agentBin := os.Getenv("SPIRE_AGENT")
	if agentBin == "" {
		agentBin = "spire-agent"
	}

	st.agentCmd = exec.Command(agentBin, "run",
		"-config", agentConfPath,
		"-socketPath", st.SocketPath,
		"-joinToken", joinToken)
	st.agentCmd.Stdout = os.Stdout
	st.agentCmd.Stderr = os.Stderr

	if err := st.agentCmd.Start(); err != nil {
		st.t.Fatalf("Failed to start SPIRE agent: %v", err)
	}

	st.agentPID = st.agentCmd.Process.Pid
	st.t.Logf("Started SPIRE agent (PID %d, socket %s)", st.agentPID, st.SocketPath)

	// Clean up agent on test completion
	st.t.Cleanup(func() {
		if st.agentCmd != nil && st.agentCmd.Process != nil {
			st.t.Logf("Stopping SPIRE agent (PID %d)", st.agentPID)
			if err := st.agentCmd.Process.Kill(); err != nil {
				st.t.Logf("Warning: failed to kill agent process: %v", err)
			}
			if err := st.agentCmd.Wait(); err != nil {
				st.t.Logf("Warning: agent process wait returned: %v", err)
			}
		}
	})

	// Give agent time to start
	time.Sleep(2 * time.Second)
}

// waitForAgent waits for the SPIRE agent to become healthy and ready to serve requests.
func (st *SPIRETest) waitForAgent() {
	st.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	logTicker := time.NewTicker(5 * time.Second)
	defer logTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			// Check if agent process is still running
			if st.agentCmd != nil && st.agentCmd.Process != nil {
				if err := st.agentCmd.Process.Signal(os.Signal(nil)); err != nil {
					st.t.Fatalf("Timed out waiting for SPIRE agent to become ready: %s (agent process has exited)", st.SocketPath)
				} else {
					st.t.Fatalf("Timed out waiting for SPIRE agent to become ready: %s (agent process still running but no socket)", st.SocketPath)
				}
			} else {
				st.t.Fatalf("Timed out waiting for SPIRE agent to become ready: %s (agent process not available)", st.SocketPath)
			}
		case <-logTicker.C:
			// Periodic progress logging
			st.t.Logf("Still waiting for SPIRE agent socket: %s", st.SocketPath)
		case <-ticker.C:
			// Check if socket exists
			if _, err := os.Stat(st.SocketPath); err == nil {
				st.t.Log("SPIRE agent is ready")
				return
			}
		}
	}
}

// createWorkloadEntry creates a workload registration entry for testing.
//
// This creates an entry that matches any workload running with the current UID,
// allowing tests to fetch SVIDs from the agent.
func (st *SPIRETest) createWorkloadEntry() {
	st.t.Helper()

	// For simplicity, create an entry that matches all workloads
	// In a real deployment, you'd have specific selectors
	spiffeID := fmt.Sprintf("spiffe://%s/test-workload", st.TrustDomain)

	serverBin := os.Getenv("SPIRE_SERVER")
	if serverBin == "" {
		serverBin = "spire-server"
	}

	// Create entry via spire-server CLI
	cmd := exec.Command(serverBin, "entry", "create",
		"-spiffeID", spiffeID,
		"-parentID", fmt.Sprintf("spiffe://%s/test-agent", st.TrustDomain),
		"-selector", fmt.Sprintf("unix:uid:%d", os.Getuid()),
		"-socketPath", st.adminAPISocket,
	)

	if output, err := cmd.CombinedOutput(); err != nil {
		st.t.Logf("Warning: Failed to create workload entry: %v\nOutput: %s", err, output)
		// Don't fail the test - entry creation might fail in some environments
	} else {
		st.t.Logf("Created workload entry: %s", spiffeID)
	}
}
