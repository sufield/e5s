package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func spiffeIDCommand(args []string) error {
	fs := flag.NewFlagSet("spiffe-id", flag.ExitOnError)
	trustDomainFlag := fs.String("trust-domain", "", "SPIRE trust domain (auto-detected if not specified)")

	fs.Usage = func() {
		fmt.Println(`Construct SPIFFE IDs from components

USAGE:
    e5s spiffe-id <type> [arguments...] [flags]

TYPES:
    k8s              Kubernetes service account
                     Args: <namespace> <service-account>
                     Trust domain auto-detected if not provided

    from-deployment  Extract from Kubernetes deployment YAML
                     Args: <deployment-file.yaml>
                     Auto-detects trust domain, namespace, and service account

    custom           Custom path
                     Args: <trust-domain> <path-component>...

FLAGS:
    --trust-domain string   SPIRE trust domain (default: auto-detect)

EXAMPLES:
    # Auto-detect trust domain (recommended)
    e5s spiffe-id k8s default default
    Output: spiffe://example.org/ns/default/sa/default

    # Explicit trust domain
    e5s spiffe-id k8s default api-client --trust-domain=example.org
    Output: spiffe://example.org/ns/default/sa/api-client

    # From deployment YAML file
    e5s spiffe-id from-deployment ./k8s/client-deployment.yaml
    Output: spiffe://example.org/ns/production/sa/api-client

    # Custom path
    e5s spiffe-id custom example.org service api-server
    Output: spiffe://example.org/service/api-server

    # Use in shell scripts
    CLIENT_ID=$(e5s spiffe-id k8s default default)
    echo "allowed_client_spiffe_id: \"$CLIENT_ID\"" >> config.yaml`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 1 {
		fs.Usage()
		return fmt.Errorf("insufficient arguments")
	}

	idType := fs.Arg(0)

	switch idType {
	case "k8s", "kubernetes":
		return handleK8sSpiffeID(fs, *trustDomainFlag)

	case "from-deployment":
		return handleFromDeployment(fs, *trustDomainFlag)

	case "custom":
		return handleCustomSpiffeID(fs)

	default:
		return fmt.Errorf("unknown type: %s (use 'k8s', 'from-deployment', or 'custom')", idType)
	}
}

func handleK8sSpiffeID(fs *flag.FlagSet, trustDomainFlag string) error {
	if fs.NArg() != 3 {
		return fmt.Errorf("k8s type requires <namespace> <service-account>")
	}

	namespace := fs.Arg(1)
	serviceAccount := fs.Arg(2)

	// Auto-detect trust domain if not provided
	trustDomain := trustDomainFlag
	if trustDomain == "" {
		td, err := autoDetectTrustDomain()
		if err != nil {
			return fmt.Errorf("failed to auto-detect trust domain (use --trust-domain flag): %w", err)
		}
		trustDomain = td
	}

	spiffeID := fmt.Sprintf("spiffe://%s/ns/%s/sa/%s", trustDomain, namespace, serviceAccount)
	fmt.Println(spiffeID)
	return nil
}

func handleFromDeployment(fs *flag.FlagSet, trustDomainFlag string) error {
	if fs.NArg() != 2 {
		return fmt.Errorf("from-deployment requires <deployment-file.yaml>")
	}

	deploymentFile := fs.Arg(1)

	// Read deployment YAML
	// #nosec G304 -- deploymentFile is from CLI argument, user controls the path
	data, err := os.ReadFile(deploymentFile)
	if err != nil {
		return fmt.Errorf("failed to read deployment file: %w", err)
	}

	content := string(data)

	// Extract namespace
	namespace := extractYAMLField(content, "namespace:")
	if namespace == "" {
		namespace = "default"
	}

	// Extract service account
	serviceAccount := extractYAMLField(content, "serviceAccountName:")
	if serviceAccount == "" {
		serviceAccount = "default"
	}

	// Auto-detect trust domain if not provided
	trustDomain := trustDomainFlag
	if trustDomain == "" {
		td, err := autoDetectTrustDomain()
		if err != nil {
			return fmt.Errorf("failed to auto-detect trust domain (use --trust-domain flag): %w", err)
		}
		trustDomain = td
	}

	spiffeID := fmt.Sprintf("spiffe://%s/ns/%s/sa/%s", trustDomain, namespace, serviceAccount)
	fmt.Println(spiffeID)
	return nil
}

func handleCustomSpiffeID(fs *flag.FlagSet) error {
	if fs.NArg() < 3 {
		return fmt.Errorf("custom type requires <trust-domain> <path-component>")
	}

	trustDomain := fs.Arg(1)
	pathComponents := fs.Args()[2:]
	path := strings.Join(pathComponents, "/")
	spiffeID := fmt.Sprintf("spiffe://%s/%s", trustDomain, path)
	fmt.Println(spiffeID)
	return nil
}

// extractYAMLField extracts a field value from YAML content
func extractYAMLField(content, field string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, field) {
			// Extract value after the field name
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 {
				value := strings.TrimSpace(parts[1])
				// Remove quotes if present
				value = strings.Trim(value, `"'`)
				return value
			}
		}
	}
	return ""
}
