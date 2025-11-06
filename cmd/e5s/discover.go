package main

import (
	"context"
	"flag"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func discoverCommand(args []string) error {
	fs := flag.NewFlagSet("discover", flag.ExitOnError)
	trustDomain := fs.String("trust-domain", "example.org", "SPIRE trust domain")
	namespace := fs.String("namespace", "", "Kubernetes namespace (default: current namespace)")

	fs.Usage = func() {
		fmt.Println(`Discover SPIFFE IDs from Kubernetes pods

USAGE:
    e5s discover <resource-type> <name> [flags]

RESOURCE TYPES:
    pod          Discover from pod name
    label        Discover from pods matching label selector
    deployment   Discover from deployment name

FLAGS:
    --trust-domain string   SPIRE trust domain (default "example.org")
    --namespace string      Kubernetes namespace (default: current namespace)

EXAMPLES:
    # Discover from pod name
    e5s discover pod e5s-client
    Output: spiffe://example.org/ns/default/sa/default

    # Discover from label selector
    e5s discover label app=api-client --namespace production
    Output: spiffe://example.org/ns/production/sa/api-client-sa

    # Discover from deployment
    e5s discover deployment web-frontend
    Output: spiffe://example.org/ns/default/sa/web-sa

    # Use in shell scripts
    CLIENT_ID=$(e5s discover pod e5s-client)
    echo "allowed_client_spiffe_id: \"$CLIENT_ID\"" >> e5s.yaml`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("insufficient arguments")
	}

	resourceType := fs.Arg(0)
	name := fs.Arg(1)

	// Check if kubectl is available
	if !isKubectlAvailable() {
		return fmt.Errorf("kubectl is not available - ensure it's installed and in PATH")
	}

	switch resourceType {
	case "pod":
		return discoverFromPod(name, *namespace, *trustDomain)
	case "label":
		return discoverFromLabel(name, *namespace, *trustDomain)
	case "deployment":
		return discoverFromDeployment(name, *namespace, *trustDomain)
	default:
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

func isKubectlAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "version", "--client", "--short")
	return cmd.Run() == nil
}

func discoverFromPod(podName, namespace, trustDomain string) error {
	// Get service account from pod
	args := []string{"get", "pod", podName}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "-o", "jsonpath={.spec.serviceAccountName},{.metadata.namespace}")

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get pod info: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) != 2 {
		return fmt.Errorf("unexpected kubectl output format")
	}

	serviceAccount := parts[0]
	podNamespace := parts[1]

	spiffeID := fmt.Sprintf("spiffe://%s/ns/%s/sa/%s", trustDomain, podNamespace, serviceAccount)
	fmt.Println(spiffeID)
	return nil
}

func discoverFromLabel(labelSelector, namespace, trustDomain string) error {
	// Get service account from first pod matching label
	args := []string{"get", "pod", "-l", labelSelector}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "-o", "jsonpath={.items[0].spec.serviceAccountName},{.items[0].metadata.namespace}")

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get pods with label %s: %w", labelSelector, err)
	}

	if len(output) == 0 {
		return fmt.Errorf("no pods found with label: %s", labelSelector)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) != 2 {
		return fmt.Errorf("unexpected kubectl output format")
	}

	serviceAccount := parts[0]
	podNamespace := parts[1]

	spiffeID := fmt.Sprintf("spiffe://%s/ns/%s/sa/%s", trustDomain, podNamespace, serviceAccount)
	fmt.Println(spiffeID)
	return nil
}

func discoverFromDeployment(deploymentName, namespace, trustDomain string) error {
	// Get service account from deployment spec
	args := []string{"get", "deployment", deploymentName}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	args = append(args, "-o", "jsonpath={.spec.template.spec.serviceAccountName},{.metadata.namespace}")

	cmd := exec.Command("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get deployment info: %w", err)
	}

	parts := strings.Split(strings.TrimSpace(string(output)), ",")
	if len(parts) != 2 {
		return fmt.Errorf("unexpected kubectl output format")
	}

	serviceAccount := parts[0]
	deployNamespace := parts[1]

	// Handle default service account
	if serviceAccount == "" {
		serviceAccount = "default"
	}

	spiffeID := fmt.Sprintf("spiffe://%s/ns/%s/sa/%s", trustDomain, deployNamespace, serviceAccount)
	fmt.Println(spiffeID)
	return nil
}
