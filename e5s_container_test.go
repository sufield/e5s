//go:build container
// +build container

package e5s_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupSpire(t *testing.T, ctx context.Context, network string, sockVol string) (server tc.Container, agent tc.Container, cleanup func()) {
	t.Helper()

	// Get absolute paths for bind mounts
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	serverConfPath := filepath.Join(wd, "testdata/spire/server.conf")
	agentConfPath := filepath.Join(wd, "testdata/spire/agent.conf")

	// SPIRE Server
	serverReq := tc.ContainerRequest{
		Image:           "ghcr.io/spiffe/spire-server:1.9.5",
		Networks:        []string{network},
		NetworkAliases:  map[string][]string{network: {"spire-server"}},
		Mounts:          tc.Mounts(tc.BindMount(serverConfPath, "/spire/server.conf")),
		Entrypoint:      []string{},
		Cmd:             []string{"/opt/spire/bin/spire-server", "run", "-config", "/spire/server.conf"},
		WaitingFor:      wait.ForLog("Starting Server APIs"),
		AlwaysPullImage: false,
	}
	srv, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: serverReq, Started: true})
	if err != nil {
		t.Fatal(err)
	}

	// Generate join token
	exitCode, tokenReader, err := srv.Exec(ctx, []string{"/opt/spire/bin/spire-server", "token", "generate", "-spiffeID", "spiffe://example.org/agent"})
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("token generate failed with exit code %d", exitCode)
	}
	tokenBytes, err := io.ReadAll(tokenReader)
	if err != nil {
		t.Fatal(err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	// SPIRE Agent
	agentReq := tc.ContainerRequest{
		Image:    "ghcr.io/spiffe/spire-agent:1.9.5",
		Networks: []string{network},
		Mounts: tc.Mounts(
			tc.BindMount(agentConfPath, "/spire/agent.conf"),
			tc.VolumeMount(sockVol, "/spire-agent"),
		),
		Privileged: true, // allow unix creds/peers
		Entrypoint: []string{},
		Cmd:        []string{"/opt/spire/bin/spire-agent", "run", "-config", "/spire/agent.conf", "-joinToken", token},
		WaitingFor: wait.ForLog("Starting Workload API"),
	}
	ag, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: agentReq, Started: true})
	if err != nil {
		t.Fatal(err)
	}

	// Wait a bit for agent to be fully ready
	time.Sleep(2 * time.Second)

	// Register workloads (UID 0 for both server and client)
	exitCode, _, err = srv.Exec(ctx, []string{
		"/opt/spire/bin/spire-server", "entry", "create",
		"-parentID", "spiffe://example.org/agent",
		"-spiffeID", "spiffe://example.org/server",
		"-selector", "unix:uid:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("server entry create failed with exit code %d", exitCode)
	}

	exitCode, _, err = srv.Exec(ctx, []string{
		"/opt/spire/bin/spire-server", "entry", "create",
		"-parentID", "spiffe://example.org/agent",
		"-spiffeID", "spiffe://example.org/client",
		"-selector", "unix:uid:0",
	})
	if err != nil {
		t.Fatal(err)
	}
	if exitCode != 0 {
		t.Fatalf("client entry create failed with exit code %d", exitCode)
	}

	cleanup = func() {
		_ = ag.Terminate(ctx)
		_ = srv.Terminate(ctx)
	}
	return srv, ag, cleanup
}

func buildImage(t *testing.T, ctx context.Context, dockerfile, tag string) {
	t.Helper()
	req := tc.ContainerRequest{
		FromDockerfile: tc.FromDockerfile{
			Context:       ".",
			Dockerfile:    dockerfile,
			PrintBuildLog: testing.Verbose(),
		},
	}
	c, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: req,
		Started:          false,
	})
	if err != nil {
		t.Fatalf("build %s failed: %v", tag, err)
	}
	// testcontainers builds the image; we can terminate the "build container"
	if c != nil {
		_ = c.Terminate(ctx)
	}
}

func TestServe_EndToEnd_withSPIRE(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container test in short mode")
	}

	ctx := context.Background()

	// Create network
	networkName := "e5snet-serve"
	net, err := tc.GenericNetwork(ctx, tc.GenericNetworkRequest{
		NetworkRequest: tc.NetworkRequest{
			Name:           networkName,
			CheckDuplicate: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer net.Remove(ctx)

	// Create volume for shared socket
	sockVol := "e5s-sock-serve"

	// Setup SPIRE
	_, _, spireCleanup := setupSpire(t, ctx, networkName, sockVol)
	defer spireCleanup()

	// Build server and client images
	t.Log("Building server image...")
	buildImage(t, ctx, "examples/container-server/Dockerfile", "e5s-server")
	t.Log("Building client image...")
	buildImage(t, ctx, "examples/container-client/Dockerfile", "e5s-client")

	// e5s-server container
	srvReq := tc.ContainerRequest{
		Image:          "e5s-server:latest",
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"e5s-server"}},
		Env:            map[string]string{"MODE": "serve"},
		Mounts: tc.Mounts(
			tc.VolumeMount(sockVol, "/spire-agent"),
		),
		ExposedPorts: []string{"8443/tcp"},
		WaitingFor:   wait.ForListeningPort("8443/tcp").WithStartupTimeout(60 * time.Second),
	}
	e5sSrv, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: srvReq, Started: true})
	if err != nil {
		t.Fatal(err)
	}
	defer e5sSrv.Terminate(ctx)

	// e5s-client container executes a single GET and prints body
	clientReq := tc.ContainerRequest{
		Image:    "e5s-client:latest",
		Networks: []string{networkName},
		Mounts: tc.Mounts(
			tc.VolumeMount(sockVol, "/spire-agent"),
		),
		WaitingFor: wait.ForExit().WithExitTimeout(30 * time.Second),
	}
	client, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: clientReq, Started: true})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Terminate(ctx)

	// Verify client output contains "Hello spiffe://"
	logs, err := client.Logs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, logs)
	output := buf.String()
	if !strings.Contains(output, "Hello spiffe://") {
		t.Fatalf("unexpected client output: %s", output)
	}
	t.Logf("✓ Client received response: %s", strings.TrimSpace(output))

	// Stop Serve by sending SIGTERM to the server container; Serve must exit cleanly.
	stopTimeout := 10 * time.Second
	err = e5sSrv.Stop(ctx, &stopTimeout)
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	t.Log("✓ Serve function tested successfully in container")
}

func TestStartSingleThread_EndToEnd_withSPIRE(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping container test in short mode")
	}

	ctx := context.Background()

	// Create network
	networkName := "e5snet-single"
	net, err := tc.GenericNetwork(ctx, tc.GenericNetworkRequest{
		NetworkRequest: tc.NetworkRequest{
			Name:           networkName,
			CheckDuplicate: true,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer net.Remove(ctx)

	// Create volume for shared socket
	sockVol := "e5s-sock-single"

	// Setup SPIRE
	_, _, spireCleanup := setupSpire(t, ctx, networkName, sockVol)
	defer spireCleanup()

	// Build images (reused from previous test if cached)
	t.Log("Building server image...")
	buildImage(t, ctx, "examples/container-server/Dockerfile", "e5s-server")
	t.Log("Building client image...")
	buildImage(t, ctx, "examples/container-client/Dockerfile", "e5s-client")

	// Start server in single-thread mode
	srvReq := tc.ContainerRequest{
		Image:          "e5s-server:latest",
		Networks:       []string{networkName},
		NetworkAliases: map[string][]string{networkName: {"e5s-server"}},
		Env:            map[string]string{"MODE": "single"},
		Mounts: tc.Mounts(
			tc.VolumeMount(sockVol, "/spire-agent"),
		),
		ExposedPorts: []string{"8443/tcp"},
		WaitingFor:   wait.ForListeningPort("8443/tcp").WithStartupTimeout(60 * time.Second),
	}
	e5sSrv, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: srvReq, Started: true})
	if err != nil {
		t.Fatal(err)
	}
	defer e5sSrv.Terminate(ctx)

	// Run client once
	clientReq := tc.ContainerRequest{
		Image:    "e5s-client:latest",
		Networks: []string{networkName},
		Mounts: tc.Mounts(
			tc.VolumeMount(sockVol, "/spire-agent"),
		),
		WaitingFor: wait.ForExit().WithExitTimeout(30 * time.Second),
	}
	client, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{ContainerRequest: clientReq, Started: true})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Terminate(ctx)

	// Assert output
	logs, err := client.Logs(ctx)
	if err != nil {
		t.Fatal(err)
	}
	buf := new(bytes.Buffer)
	_, _ = io.Copy(buf, logs)
	output := buf.String()
	if !strings.Contains(output, "Hello spiffe://") {
		t.Fatalf("unexpected client output: %s", output)
	}
	t.Logf("✓ Client received response: %s", strings.TrimSpace(output))

	// For StartSingleThread, stop the server by stopping the container; function should return.
	stopTimeout := 10 * time.Second
	err = e5sSrv.Stop(ctx, &stopTimeout)
	if err != nil {
		t.Errorf("Failed to stop server: %v", err)
	}

	t.Log("✓ StartSingleThread function tested successfully in container")
}
