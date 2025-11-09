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
	trustDomain := fs.String("trust-domain", "", "SPIRE trust domain (default: auto-detect from cluster)")
	namespace := fs.String("namespace", "", "Kubernetes namespace (default: current namespace)")

	fs.Usage = func() {
		fmt.Println(`Discover SPIFFE IDs from Kubernetes pods

USAGE:
    e5s discover <resource-type> <name> [flags]

RESOURCE TYPES:
    trust-domain Discover SPIRE trust domain from cluster
    pod          Discover from pod name
    label        Discover from pods matching label selector
    deployment   Discover from deployment name

FLAGS:
    --trust-domain string   SPIRE trust domain (default: auto-detect)
    --namespace string      Kubernetes namespace (default: current namespace)

EXAMPLES:
    # Discover trust domain from SPIRE installation
    e5s discover trust-domain
    Output: example.org

    # Discover from pod name (auto-detects trust domain)
    e5s discover pod e5s-client
    Output: spiffe://example.org/ns/default/sa/default

    # Discover from label selector
    e5s discover label app=api-client --namespace production
    Output: spiffe://example.org/ns/production/sa/api-client-sa

    # Discover from deployment
    e5s discover deployment web-frontend
    Output: spiffe://example.org/ns/default/sa/web-sa

    # Use in shell scripts
    TRUST_DOMAIN=$(e5s discover trust-domain)
    CLIENT_ID=$(e5s discover pod e5s-client)
    echo "allowed_client_spiffe_id: \"$CLIENT_ID\"" >> e5s.yaml`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("insufficient arguments")
	}

	resourceType := fs.Arg(0)

	// Handle trust-domain discovery (no second argument needed)
	if resourceType == "trust-domain" {
		return discoverTrustDomain()
	}

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("insufficient arguments for %s", resourceType)
	}

	name := fs.Arg(1)

	// Check if kubectl is available
	if !isKubectlAvailable() {
		return fmt.Errorf("kubectl is not available - ensure it's installed and in PATH")
	}

	// Auto-detect trust domain if not provided
	td := *trustDomain
	if td == "" {
		detectedTD, err := autoDetectTrustDomain()
		if err != nil {
			return fmt.Errorf("failed to auto-detect trust domain (use --trust-domain flag): %w", err)
		}
		td = detectedTD
	}

	switch resourceType {
	case "pod":
		return discoverFromPod(name, *namespace, td)
	case "label":
		return discoverFromLabel(name, *namespace, td)
	case "deployment":
		return discoverFromDeployment(name, *namespace, td)
	default:
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

func isKubectlAvailable() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "kubectl", "version", "--client")
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

// discoverTrustDomain discovers the SPIRE trust domain from the cluster
func discoverTrustDomain() error {
	td, err := autoDetectTrustDomain()
	if err != nil {
		return err
	}
	fmt.Println(td)
	return nil
}

// autoDetectTrustDomain attempts to auto-detect the SPIRE trust domain
func autoDetectTrustDomain() (string, error) {
	// Try multiple methods to detect trust domain

	// Method 1: Check Helm release values for SPIRE
	if td, err := getTrustDomainFromHelm(); err == nil && td != "" {
		return td, nil
	}

	// Method 2: Check SPIRE server configmap
	if td, err := getTrustDomainFromConfigMap(); err == nil && td != "" {
		return td, nil
	}

	// Method 3: Check environment variable
	// This would be set in CI/CD or local development
	// We could support E5S_TRUST_DOMAIN env var in future

	return "", fmt.Errorf("could not auto-detect trust domain - tried Helm and ConfigMap methods")
}

// getTrustDomainFromHelm tries to get trust domain from Helm release
func getTrustDomainFromHelm() (string, error) {
	// Check if helm is available
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if exec.CommandContext(ctx, "helm", "version").Run() != nil {
		return "", fmt.Errorf("helm not available")
	}

	// Try to get SPIRE Helm values
	cmd := exec.Command("helm", "get", "values", "spire", "-n", "spire", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get helm values: %w", err)
	}

	// Parse JSON to extract trustDomain
	// Simple string search for "trustDomain":"value"
	outputStr := string(output)
	trustDomainPrefix := `"trustDomain":"`
	idx := strings.Index(outputStr, trustDomainPrefix)
	if idx == -1 {
		return "", fmt.Errorf("trustDomain not found in helm values")
	}

	// Extract the value after "trustDomain":"
	start := idx + len(trustDomainPrefix)
	end := strings.Index(outputStr[start:], `"`)
	if end == -1 {
		return "", fmt.Errorf("malformed trustDomain value")
	}

	return outputStr[start : start+end], nil
}

// getTrustDomainFromConfigMap tries to get trust domain from SPIRE server configmap
func getTrustDomainFromConfigMap() (string, error) {
	// Try to get SPIRE server configmap
	cmd := exec.Command("kubectl", "get", "configmap", "-n", "spire", "-l", "app.kubernetes.io/name=server", "-o", "yaml")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get SPIRE configmap: %w", err)
	}

	// Search for trust_domain in the configmap
	outputStr := string(output)
	for _, line := range strings.Split(outputStr, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "trust_domain") {
			// Parse: trust_domain = "example.org"
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				td := strings.Trim(strings.TrimSpace(parts[1]), `"`)
				return td, nil
			}
		}
	}

	return "", fmt.Errorf("trust_domain not found in SPIRE configmap")
}
