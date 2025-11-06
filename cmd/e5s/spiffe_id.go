package main

import (
	"flag"
	"fmt"
	"strings"
)

func spiffeIDCommand(args []string) error {
	fs := flag.NewFlagSet("spiffe-id", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Println(`Construct SPIFFE IDs from components

USAGE:
    e5s spiffe-id <type> <trust-domain> [components...]

TYPES:
    k8s          Kubernetes service account (requires: namespace, service-account)
    custom       Custom path (requires: path components)

EXAMPLES:
    # Kubernetes service account
    e5s spiffe-id k8s example.org default api-client
    Output: spiffe://example.org/ns/default/sa/api-client

    # Custom path
    e5s spiffe-id custom example.org service api-server
    Output: spiffe://example.org/service/api-server

    # Use in shell scripts
    ALLOWED_CLIENT_ID=$(e5s spiffe-id k8s example.org default api-client)
    echo "allowed_client_spiffe_id: \"$ALLOWED_CLIENT_ID\"" >> config.yaml`)
	}

	if err := fs.Parse(args); err != nil {
		return err
	}

	if fs.NArg() < 2 {
		fs.Usage()
		return fmt.Errorf("insufficient arguments")
	}

	idType := fs.Arg(0)
	trustDomain := fs.Arg(1)

	switch idType {
	case "k8s", "kubernetes":
		if fs.NArg() != 4 {
			return fmt.Errorf("k8s type requires <trust-domain> <namespace> <service-account>")
		}
		namespace := fs.Arg(2)
		serviceAccount := fs.Arg(3)
		spiffeID := fmt.Sprintf("spiffe://%s/ns/%s/sa/%s", trustDomain, namespace, serviceAccount)
		fmt.Println(spiffeID)

	case "custom":
		if fs.NArg() < 3 {
			return fmt.Errorf("custom type requires <trust-domain> <path-component>")
		}
		pathComponents := fs.Args()[2:]
		path := strings.Join(pathComponents, "/")
		spiffeID := fmt.Sprintf("spiffe://%s/%s", trustDomain, path)
		fmt.Println(spiffeID)

	default:
		return fmt.Errorf("unknown type: %s (use 'k8s' or 'custom')", idType)
	}

	return nil
}
