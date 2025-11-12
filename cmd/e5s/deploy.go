package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"helm.sh/helm/v3/pkg/cli"
	"helm.sh/helm/v3/pkg/getter"
	"helm.sh/helm/v3/pkg/repo"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/kind/pkg/cluster"
	"sigs.k8s.io/kind/pkg/cmd"
)

func deployCommand(args []string) error {
	if len(args) < 1 {
		printDeployUsage()
		return fmt.Errorf("no subcommand specified")
	}

	subcommand := args[0]
	subArgs := args[1:]

	switch subcommand {
	case "cluster":
		return deployClusterCommand(subArgs)
	case "spire":
		return deploySpireCommand(subArgs)
	case "app":
		return deployAppCommand(subArgs)
	case "test":
		return deployTestCommand(subArgs)
	case "help", "-h", "--help":
		printDeployUsage()
		return nil
	default:
		printDeployUsage()
		return fmt.Errorf("unknown deploy subcommand: %s", subcommand)
	}
}

func printDeployUsage() {
	fmt.Fprintf(os.Stderr, `Deploy and manage e5s test environments

USAGE:
    e5s deploy <subcommand> [arguments] [flags]

SUBCOMMANDS:
    cluster     Manage local Kubernetes clusters (Kind)
    spire       Install and manage SPIRE using Helm
    app         Deploy e5s applications using Helm chart
    test        Run integration tests and verify mTLS

EXAMPLES:
    # Complete workflow
    e5s deploy cluster create --name e5s-test --wait 60s
    e5s deploy spire install --trust-domain example.org
    e5s deploy app install --chart-path chart/e5s-demo
    e5s deploy test run

    # Verify mTLS communication
    e5s deploy test verify

    # Check status
    e5s deploy spire status
    e5s deploy app status

    # Clean up
    e5s deploy app uninstall
    e5s deploy spire uninstall
    e5s deploy cluster delete --name e5s-test

Run 'e5s deploy <subcommand> --help' for more information on a subcommand.
`)
}

// deployClusterCommand handles cluster management subcommands
func deployClusterCommand(args []string) error {
	if len(args) < 1 {
		printClusterUsage()
		return fmt.Errorf("no cluster subcommand specified")
	}

	action := args[0]
	actionArgs := args[1:]

	switch action {
	case "create":
		return deployClusterCreate(actionArgs)
	case "delete":
		return deployClusterDelete(actionArgs)
	case "help", "-h", "--help":
		printClusterUsage()
		return nil
	default:
		printClusterUsage()
		return fmt.Errorf("unknown cluster action: %s", action)
	}
}

func printClusterUsage() {
	fmt.Fprintf(os.Stderr, `Manage local Kubernetes clusters using Kind

USAGE:
    e5s deploy cluster <action> [flags]

ACTIONS:
    create      Create a new Kind cluster
    delete      Delete a Kind cluster

FLAGS:
    --name string   Cluster name (default "e5s-test")

EXAMPLES:
    # Create cluster with default name
    e5s deploy cluster create

    # Create cluster with custom name
    e5s deploy cluster create --name e5s-prod

    # Delete cluster
    e5s deploy cluster delete --name e5s-prod
`)
}

func deployClusterCreate(args []string) error {
	fs := flag.NewFlagSet("deploy cluster create", flag.ExitOnError)
	clusterName := fs.String("name", "e5s-test", "Cluster name")
	wait := fs.Duration("wait", 0, "Wait for control plane to be ready")

	fs.Usage = printClusterUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Creating Kind cluster: %s\n", *clusterName)

	// Create Kind cluster provider
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(cmd.NewLogger()),
	)

	// Create the cluster
	if err := provider.Create(
		*clusterName,
		cluster.CreateWithWaitForReady(*wait),
	); err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	fmt.Printf("✓ Cluster '%s' created successfully\n", *clusterName)
	fmt.Printf("\nTo use this cluster:\n")
	fmt.Printf("  kubectl cluster-info --context kind-%s\n", *clusterName)
	return nil
}

func deployClusterDelete(args []string) error {
	fs := flag.NewFlagSet("deploy cluster delete", flag.ExitOnError)
	clusterName := fs.String("name", "e5s-test", "Cluster name")
	kubeconfigPath := fs.String("kubeconfig", "", "Path to kubeconfig (defaults to standard location)")

	fs.Usage = printClusterUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Deleting Kind cluster: %s\n", *clusterName)

	// Create Kind cluster provider
	provider := cluster.NewProvider(
		cluster.ProviderWithLogger(cmd.NewLogger()),
	)

	// Delete the cluster
	if err := provider.Delete(*clusterName, *kubeconfigPath); err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	fmt.Printf("✓ Cluster '%s' deleted successfully\n", *clusterName)
	return nil
}

// deploySpireCommand handles SPIRE installation (Phase 2)
func deploySpireCommand(args []string) error {
	if len(args) < 1 {
		printSpireUsage()
		return fmt.Errorf("no subcommand specified")
	}

	action := args[0]
	actionArgs := args[1:]

	switch action {
	case "install":
		return deploySpireInstall(actionArgs)
	case "upgrade":
		return deploySpireUpgrade(actionArgs)
	case "uninstall":
		return deploySpireUninstall(actionArgs)
	case "status":
		return deploySpireStatus(actionArgs)
	case "help", "-h", "--help":
		printSpireUsage()
		return nil
	default:
		printSpireUsage()
		return fmt.Errorf("unknown spire action: %s", action)
	}
}

func printSpireUsage() {
	fmt.Fprintf(os.Stderr, `Install and manage SPIRE using Helm

USAGE:
    e5s deploy spire <action> [flags]

ACTIONS:
    install     Install SPIRE using Helm charts
    upgrade     Upgrade SPIRE release
    uninstall   Uninstall SPIRE release
    status      Show SPIRE deployment status

FLAGS (install/upgrade):
    --release-name string     Helm release name (default "spire")
    --namespace string        Kubernetes namespace (default "spire-system")
    --trust-domain string     SPIFFE trust domain (default "example.org")
    --wait                    Wait for deployment to complete (default true)
    --timeout duration        Wait timeout (default 5m)

EXAMPLES:
    # Install SPIRE with default settings
    e5s deploy spire install

    # Install with custom trust domain
    e5s deploy spire install --trust-domain prod.company.com

    # Check status
    e5s deploy spire status

    # Uninstall
    e5s deploy spire uninstall
`)
}

func deploySpireInstall(args []string) error {
	fs := flag.NewFlagSet("deploy spire install", flag.ExitOnError)
	releaseName := fs.String("release-name", "spire", "Helm release name")
	namespace := fs.String("namespace", "spire-system", "Kubernetes namespace")
	trustDomain := fs.String("trust-domain", "example.org", "SPIFFE trust domain")
	wait := fs.Bool("wait", true, "Wait for deployment to complete")
	timeout := fs.Duration("timeout", 5*time.Minute, "Wait timeout")

	fs.Usage = printSpireUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Installing SPIRE (release: %s, namespace: %s, trust-domain: %s)\n",
		*releaseName, *namespace, *trustDomain)

	// Initialize Helm settings
	settings := cli.New()
	settings.SetNamespace(*namespace)

	// Create Helm action configuration
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), *namespace,
		os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
			fmt.Printf(format+"\n", v...)
		}); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	// Add SPIRE Helm repository
	fmt.Println("Adding SPIRE Helm repository...")
	if err := addHelmRepo(settings, "spiffe", "https://spiffe.github.io/helm-charts-hardened/"); err != nil {
		return fmt.Errorf("failed to add Helm repository: %w", err)
	}

	// Install using Helm
	client := action.NewInstall(actionConfig)
	client.ReleaseName = *releaseName
	client.Namespace = *namespace
	client.CreateNamespace = true
	client.Wait = *wait
	client.Timeout = *timeout

	// Load chart
	chartPath, err := client.ChartPathOptions.LocateChart("spiffe/spire", settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	// Set values
	values := map[string]interface{}{
		"global": map[string]interface{}{
			"spire": map[string]interface{}{
				"trustDomain": *trustDomain,
			},
		},
	}

	// Run install
	release, err := client.Run(chart, values)
	if err != nil {
		return fmt.Errorf("failed to install SPIRE: %w", err)
	}

	fmt.Printf("✓ SPIRE installed successfully (release: %s, status: %s)\n",
		release.Name, release.Info.Status)
	fmt.Printf("\nTo verify installation:\n")
	fmt.Printf("  kubectl get pods -n %s\n", *namespace)
	return nil
}

func deploySpireUpgrade(args []string) error {
	fs := flag.NewFlagSet("deploy spire upgrade", flag.ExitOnError)
	releaseName := fs.String("release-name", "spire", "Helm release name")
	namespace := fs.String("namespace", "spire-system", "Kubernetes namespace")
	wait := fs.Bool("wait", true, "Wait for upgrade to complete")
	timeout := fs.Duration("timeout", 5*time.Minute, "Wait timeout")

	fs.Usage = printSpireUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Upgrading SPIRE (release: %s, namespace: %s)\n", *releaseName, *namespace)

	settings := cli.New()
	settings.SetNamespace(*namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), *namespace,
		os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
			fmt.Printf(format+"\n", v...)
		}); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	client := action.NewUpgrade(actionConfig)
	client.Namespace = *namespace
	client.Wait = *wait
	client.Timeout = *timeout

	chartPath, err := client.ChartPathOptions.LocateChart("spiffe/spire", settings)
	if err != nil {
		return fmt.Errorf("failed to locate chart: %w", err)
	}

	chart, err := loader.Load(chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart: %w", err)
	}

	release, err := client.Run(*releaseName, chart, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade SPIRE: %w", err)
	}

	fmt.Printf("✓ SPIRE upgraded successfully (release: %s, status: %s)\n",
		release.Name, release.Info.Status)
	return nil
}

func deploySpireUninstall(args []string) error {
	fs := flag.NewFlagSet("deploy spire uninstall", flag.ExitOnError)
	releaseName := fs.String("release-name", "spire", "Helm release name")
	namespace := fs.String("namespace", "spire-system", "Kubernetes namespace")

	fs.Usage = printSpireUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Uninstalling SPIRE (release: %s, namespace: %s)\n", *releaseName, *namespace)

	settings := cli.New()
	settings.SetNamespace(*namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), *namespace,
		os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
			fmt.Printf(format+"\n", v...)
		}); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	client := action.NewUninstall(actionConfig)
	_, err := client.Run(*releaseName)
	if err != nil {
		return fmt.Errorf("failed to uninstall SPIRE: %w", err)
	}

	fmt.Printf("✓ SPIRE uninstalled successfully\n")
	return nil
}

func deploySpireStatus(args []string) error {
	fs := flag.NewFlagSet("deploy spire status", flag.ExitOnError)
	releaseName := fs.String("release-name", "spire", "Helm release name")
	namespace := fs.String("namespace", "spire-system", "Kubernetes namespace")

	fs.Usage = printSpireUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	settings := cli.New()
	settings.SetNamespace(*namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), *namespace,
		os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
			fmt.Printf(format+"\n", v...)
		}); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	client := action.NewStatus(actionConfig)
	release, err := client.Run(*releaseName)
	if err != nil {
		return fmt.Errorf("failed to get SPIRE status: %w", err)
	}

	fmt.Printf("Release: %s\n", release.Name)
	fmt.Printf("Namespace: %s\n", release.Namespace)
	fmt.Printf("Status: %s\n", release.Info.Status)
	fmt.Printf("Version: %d\n", release.Version)
	fmt.Printf("Last deployed: %s\n", release.Info.LastDeployed.Format(time.RFC3339))
	return nil
}

// addHelmRepo adds a Helm repository
func addHelmRepo(settings *cli.EnvSettings, name, url string) error {
	repoFile := settings.RepositoryConfig
	repoDir := filepath.Dir(repoFile)

	// Create repo directory if it doesn't exist
	if err := os.MkdirAll(repoDir, 0o750); err != nil {
		return fmt.Errorf("failed to create repository directory: %w", err)
	}

	// Create repo entry
	entry := &repo.Entry{
		Name: name,
		URL:  url,
	}

	// Download index
	r, err := repo.NewChartRepository(entry, getter.All(settings))
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	if _, err := r.DownloadIndexFile(); err != nil {
		return fmt.Errorf("failed to download repository index: %w", err)
	}

	// Load existing repo file
	repoFileData, err := repo.LoadFile(repoFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to load repository file: %w", err)
	}
	if os.IsNotExist(err) {
		repoFileData = repo.NewFile()
	}

	// Add or update entry
	repoFileData.Update(entry)

	// Save repo file
	if err := repoFileData.WriteFile(repoFile, 0o644); err != nil {
		return fmt.Errorf("failed to write repository file: %w", err)
	}

	fmt.Printf("✓ Repository '%s' added: %s\n", name, url)
	return nil
}

// deployAppCommand handles application deployment (Phase 3)
func deployAppCommand(args []string) error {
	if len(args) < 1 {
		printAppUsage()
		return fmt.Errorf("no subcommand specified")
	}

	action := args[0]
	actionArgs := args[1:]

	switch action {
	case "install":
		return deployAppInstall(actionArgs)
	case "uninstall":
		return deployAppUninstall(actionArgs)
	case "status":
		return deployAppStatus(actionArgs)
	case "help", "-h", "--help":
		printAppUsage()
		return nil
	default:
		printAppUsage()
		return fmt.Errorf("unknown app action: %s", action)
	}
}

func printAppUsage() {
	fmt.Fprintf(os.Stderr, `Deploy e5s applications using Helm chart

USAGE:
    e5s deploy app <action> [flags]

ACTIONS:
    install     Deploy e5s server and client applications
    uninstall   Remove e5s applications
    status      Show application deployment status

FLAGS (install):
    --release-name string   Helm release name (default "e5s-demo")
    --namespace string      Kubernetes namespace (default "default")
    --chart-path string     Path to Helm chart (default "chart/e5s-demo")
    --server-image string   Server container image (default "e5s-server:dev")
    --client-image string   Client container image (default "e5s-client:dev")
    --trust-domain string   SPIFFE trust domain (default "example.org")
    --wait                  Wait for deployment to complete (default true)
    --timeout duration      Wait timeout (default 5m)

EXAMPLES:
    # Deploy using local chart
    e5s deploy app install

    # Deploy with custom images
    e5s deploy app install \
      --server-image myregistry/e5s-server:v1.0.0 \
      --client-image myregistry/e5s-client:v1.0.0

    # Check status
    e5s deploy app status

    # Uninstall
    e5s deploy app uninstall
`)
}

func deployAppInstall(args []string) error {
	fs := flag.NewFlagSet("deploy app install", flag.ExitOnError)
	releaseName := fs.String("release-name", "e5s-demo", "Helm release name")
	namespace := fs.String("namespace", "default", "Kubernetes namespace")
	chartPath := fs.String("chart-path", "chart/e5s-demo", "Path to Helm chart")
	serverImage := fs.String("server-image", "e5s-server:dev", "Server container image")
	_ = fs.String("client-image", "e5s-client:dev", "Client container image") // For future use
	trustDomain := fs.String("trust-domain", "example.org", "SPIFFE trust domain")
	wait := fs.Bool("wait", true, "Wait for deployment to complete")
	timeout := fs.Duration("timeout", 5*time.Minute, "Wait timeout")

	fs.Usage = printAppUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Deploying e5s applications (release: %s, namespace: %s)\n",
		*releaseName, *namespace)

	// Initialize Helm settings
	settings := cli.New()
	settings.SetNamespace(*namespace)

	// Create Helm action configuration
	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), *namespace,
		os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
			fmt.Printf(format+"\n", v...)
		}); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	// Install using Helm
	client := action.NewInstall(actionConfig)
	client.ReleaseName = *releaseName
	client.Namespace = *namespace
	client.CreateNamespace = false // Assume namespace exists
	client.Wait = *wait
	client.Timeout = *timeout

	// Load local chart
	chart, err := loader.Load(*chartPath)
	if err != nil {
		return fmt.Errorf("failed to load chart from %s: %w", *chartPath, err)
	}

	// Set values
	values := map[string]interface{}{
		"server": map[string]interface{}{
			"enabled": true,
			"image": map[string]interface{}{
				"repository": parseImageRepo(*serverImage),
				"tag":        parseImageTag(*serverImage),
				"pullPolicy": "Never", // For local images
			},
		},
		"client": map[string]interface{}{
			"enabled": false, // Don't deploy client as a pod, use jobs instead
		},
		"global": map[string]interface{}{
			"trustDomain": *trustDomain,
		},
	}

	// Run install
	release, err := client.Run(chart, values)
	if err != nil {
		return fmt.Errorf("failed to deploy applications: %w", err)
	}

	fmt.Printf("✓ Applications deployed successfully (release: %s, status: %s)\n",
		release.Name, release.Info.Status)
	fmt.Printf("\nTo verify deployment:\n")
	fmt.Printf("  kubectl get pods -n %s -l app.kubernetes.io/instance=%s\n", *namespace, *releaseName)
	return nil
}

func deployAppUninstall(args []string) error {
	fs := flag.NewFlagSet("deploy app uninstall", flag.ExitOnError)
	releaseName := fs.String("release-name", "e5s-demo", "Helm release name")
	namespace := fs.String("namespace", "default", "Kubernetes namespace")

	fs.Usage = printAppUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	fmt.Printf("Uninstalling e5s applications (release: %s, namespace: %s)\n", *releaseName, *namespace)

	settings := cli.New()
	settings.SetNamespace(*namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), *namespace,
		os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
			fmt.Printf(format+"\n", v...)
		}); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	client := action.NewUninstall(actionConfig)
	_, err := client.Run(*releaseName)
	if err != nil {
		return fmt.Errorf("failed to uninstall applications: %w", err)
	}

	fmt.Printf("✓ Applications uninstalled successfully\n")
	return nil
}

func deployAppStatus(args []string) error {
	fs := flag.NewFlagSet("deploy app status", flag.ExitOnError)
	releaseName := fs.String("release-name", "e5s-demo", "Helm release name")
	namespace := fs.String("namespace", "default", "Kubernetes namespace")

	fs.Usage = printAppUsage
	if err := fs.Parse(args); err != nil {
		return err
	}

	settings := cli.New()
	settings.SetNamespace(*namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), *namespace,
		os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {
			fmt.Printf(format+"\n", v...)
		}); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	client := action.NewStatus(actionConfig)
	release, err := client.Run(*releaseName)
	if err != nil {
		return fmt.Errorf("failed to get app status: %w", err)
	}

	fmt.Printf("Release: %s\n", release.Name)
	fmt.Printf("Namespace: %s\n", release.Namespace)
	fmt.Printf("Status: %s\n", release.Info.Status)
	fmt.Printf("Version: %d\n", release.Version)
	fmt.Printf("Last deployed: %s\n", release.Info.LastDeployed.Format(time.RFC3339))
	return nil
}

// parseImageRepo extracts repository from image string
func parseImageRepo(image string) string {
	parts := strings.Split(image, ":")
	return parts[0]
}

// parseImageTag extracts tag from image string
func parseImageTag(image string) string {
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		return parts[1]
	}
	return "latest"
}

// deployTestCommand handles integration testing (Phase 4)
func deployTestCommand(args []string) error {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: e5s deploy test <subcommand>")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Available subcommands:")
		fmt.Fprintln(os.Stderr, "  run      Run end-to-end integration tests")
		fmt.Fprintln(os.Stderr, "  verify   Verify mTLS communication between client and server")
		return fmt.Errorf("no subcommand specified")
	}

	subcommand := args[0]
	switch subcommand {
	case "run":
		return deployTestRun(args[1:])
	case "verify":
		return deployTestVerify(args[1:])
	default:
		return fmt.Errorf("unknown test subcommand: %s", subcommand)
	}
}

// deployTestRun runs complete end-to-end integration tests
func deployTestRun(args []string) error {
	fs := flag.NewFlagSet("deploy test run", flag.ExitOnError)
	namespace := fs.String("namespace", "default", "Kubernetes namespace")
	releaseName := fs.String("release-name", "e5s-demo", "Helm release name")
	trustDomain := fs.String("trust-domain", "demo.e5s.io", "SPIFFE trust domain")
	cleanup := fs.Bool("cleanup", true, "Clean up test resources after completion")

	if err := fs.Parse(args); err != nil {
		return err
	}

	_ = trustDomain // Reserved for future use

	fmt.Println("🧪 Running e5s integration tests...")
	fmt.Println("")

	// Step 1: Check if server is deployed
	fmt.Println("1. Checking server deployment...")
	settings := cli.New()
	settings.SetNamespace(*namespace)

	actionConfig := new(action.Configuration)
	if err := actionConfig.Init(settings.RESTClientGetter(), *namespace,
		os.Getenv("HELM_DRIVER"), func(format string, v ...interface{}) {}); err != nil {
		return fmt.Errorf("failed to initialize Helm: %w", err)
	}

	statusClient := action.NewStatus(actionConfig)
	release, err := statusClient.Run(*releaseName)
	if err != nil {
		fmt.Println("   ❌ Server not deployed")
		return fmt.Errorf("server not deployed (run 'e5s deploy app install' first): %w", err)
	}
	fmt.Printf("   ✓ Server deployed (release: %s, status: %s)\n", release.Name, release.Info.Status)
	fmt.Println("")

	// Step 2: Run mTLS verification
	fmt.Println("2. Running mTLS verification test...")
	if err := runMTLSTest(*namespace); err != nil {
		fmt.Println("   ❌ mTLS verification failed")
		return err
	}
	fmt.Println("   ✓ mTLS communication successful")
	fmt.Println("")

	// Step 3: Cleanup if requested
	if *cleanup {
		fmt.Println("3. Cleaning up test resources...")
		if err := cleanupTestResources(*namespace); err != nil {
			fmt.Printf("   ⚠ Warning: cleanup failed: %v\n", err)
		} else {
			fmt.Println("   ✓ Test resources cleaned up")
		}
		fmt.Println("")
	}

	fmt.Println("✅ All integration tests passed!")
	return nil
}

// deployTestVerify verifies mTLS communication
func deployTestVerify(args []string) error {
	fs := flag.NewFlagSet("deploy test verify", flag.ExitOnError)
	namespace := fs.String("namespace", "default", "Kubernetes namespace")
	trustDomain := fs.String("trust-domain", "demo.e5s.io", "SPIFFE trust domain")
	cleanup := fs.Bool("cleanup", true, "Clean up test client after verification")

	if err := fs.Parse(args); err != nil {
		return err
	}

	_ = trustDomain // Reserved for future use

	fmt.Println("🔍 Verifying mTLS communication...")
	fmt.Println("")

	if err := runMTLSTest(*namespace); err != nil {
		fmt.Println("❌ mTLS verification failed")
		return err
	}

	fmt.Println("✅ mTLS communication verified successfully!")
	fmt.Println("")

	if *cleanup {
		fmt.Println("Cleaning up test client...")
		if err := cleanupTestResources(*namespace); err != nil {
			fmt.Printf("⚠ Warning: cleanup failed: %v\n", err)
		} else {
			fmt.Println("✓ Cleanup complete")
		}
	}

	return nil
}

// runMTLSTest creates a test client pod and verifies mTLS communication
func runMTLSTest(namespace string) error {
	// Create Kubernetes clientset
	config, err := getKubeConfig()
	if err != nil {
		return fmt.Errorf("failed to get kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes client: %w", err)
	}

	ctx := context.Background()

	// Create test client pod
	testPodName := "e5s-test-client"

	fmt.Println("   Creating test client pod...")
	pod := createTestClientPod(testPodName, namespace)

	// Delete existing test pod if it exists
	_ = clientset.CoreV1().Pods(namespace).Delete(ctx, testPodName, metav1.DeleteOptions{})
	time.Sleep(2 * time.Second)

	_, err = clientset.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create test client pod: %w", err)
	}

	// Wait for pod to complete
	fmt.Println("   Waiting for test client to complete...")
	if err := waitForPodCompletion(clientset, namespace, testPodName, 60*time.Second); err != nil {
		// Get logs even on failure for debugging
		logs := getPodLogs(clientset, namespace, testPodName)
		if logs != "" {
			fmt.Println("   Pod logs:")
			fmt.Println(logs)
		}
		return fmt.Errorf("test client failed: %w", err)
	}

	// Get pod logs
	logs := getPodLogs(clientset, namespace, testPodName)
	if logs == "" {
		return fmt.Errorf("no logs from test client")
	}

	fmt.Println("   Test client output:")
	for _, line := range strings.Split(strings.TrimSpace(logs), "\n") {
		fmt.Printf("     %s\n", line)
	}

	// Check if response contains expected greeting
	if !strings.Contains(logs, "Hello, spiffe://") {
		return fmt.Errorf("unexpected response from server (mTLS may have failed)")
	}

	return nil
}

// createTestClientPod creates a pod spec for testing mTLS
func createTestClientPod(name, namespace string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app":                          "e5s-test-client",
				"app.kubernetes.io/managed-by": "e5s-cli",
			},
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:            "client",
					Image:           "e5s-client:dev",
					ImagePullPolicy: corev1.PullNever,
					Env: []corev1.EnvVar{
						{
							Name:  "SERVER_URL",
							Value: "https://e5s-demo-server:8443/hello",
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "spire-agent-socket",
							MountPath: "/spire-agent-socket",
							ReadOnly:  true,
						},
						{
							Name:      "config",
							MountPath: "/app/e5s.yaml",
							SubPath:   "e5s.yaml",
						},
					},
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "spire-agent-socket",
					VolumeSource: corev1.VolumeSource{
						CSI: &corev1.CSIVolumeSource{
							Driver:   "csi.spiffe.io",
							ReadOnly: ptrBool(true),
						},
					},
				},
				{
					Name: "config",
					VolumeSource: corev1.VolumeSource{
						ConfigMap: &corev1.ConfigMapVolumeSource{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "e5s-test-client-config",
							},
						},
					},
				},
			},
		},
	}
}

// waitForPodCompletion waits for a pod to complete (succeed or fail)
func waitForPodCompletion(clientset *kubernetes.Clientset, namespace, podName string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for pod to complete")
		case <-ticker.C:
			pod, err := clientset.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("failed to get pod status: %w", err)
			}

			switch pod.Status.Phase {
			case corev1.PodSucceeded:
				return nil
			case corev1.PodFailed:
				return fmt.Errorf("pod failed")
			case corev1.PodRunning:
				// Check if container has terminated
				if len(pod.Status.ContainerStatuses) > 0 {
					status := pod.Status.ContainerStatuses[0]
					if status.State.Terminated != nil {
						if status.State.Terminated.ExitCode == 0 {
							return nil
						}
						return fmt.Errorf("container exited with code %d", status.State.Terminated.ExitCode)
					}
				}
			}
		}
	}
}

// getPodLogs retrieves logs from a pod
func getPodLogs(clientset *kubernetes.Clientset, namespace, podName string) string {
	req := clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{})
	logs, err := req.Stream(context.Background())
	if err != nil {
		return ""
	}
	defer logs.Close()

	buf := new(strings.Builder)
	_, _ = io.Copy(buf, logs)
	return buf.String()
}

// cleanupTestResources removes test client pod and configmap
func cleanupTestResources(namespace string) error {
	config, err := getKubeConfig()
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	ctx := context.Background()

	// Delete test pod
	_ = clientset.CoreV1().Pods(namespace).Delete(ctx, "e5s-test-client", metav1.DeleteOptions{})

	// Delete test configmap (if it exists)
	_ = clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, "e5s-test-client-config", metav1.DeleteOptions{})

	return nil
}

// getKubeConfig returns Kubernetes REST config
func getKubeConfig() (*rest.Config, error) {
	settings := cli.New()
	return settings.RESTClientGetter().ToRESTConfig()
}

// ptrBool returns a pointer to a bool
func ptrBool(b bool) *bool {
	return &b
}
