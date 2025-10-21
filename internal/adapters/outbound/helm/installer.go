//go:build dev

package helm

import (
	"fmt"
	"os/exec"
	"time"
)

// Installer provides Helm installation functionality for dev environments
type Installer struct {
	helmPath     string
	helmfilePath string
	kubectlPath  string
	infraDir     string
	timeout      time.Duration
	dryRun       bool
	debug        bool
}

// NewInstaller creates a new Helm installer
func NewInstaller(infraDir string) (*Installer, error) {
	// Find helm binary
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return nil, fmt.Errorf("helm not found in PATH: %w", err)
	}

	// Find helmfile (optional)
	helmfilePath, _ := exec.LookPath("helmfile")

	// Find kubectl
	kubectlPath, err := exec.LookPath("kubectl")
	if err != nil {
		return nil, fmt.Errorf("kubectl not found in PATH: %w", err)
	}

	return &Installer{
		helmPath:     helmPath,
		helmfilePath: helmfilePath,
		kubectlPath:  kubectlPath,
		infraDir:     infraDir,
		timeout:      5 * time.Minute,
		dryRun:       false,
		debug:        false,
	}, nil
}

// SetTimeout sets the operation timeout
func (i *Installer) SetTimeout(timeout time.Duration) {
	i.timeout = timeout
}

// SetDryRun enables dry-run mode
func (i *Installer) SetDryRun(dryRun bool) {
	i.dryRun = dryRun
}

// SetDebug enables debug output
func (i *Installer) SetDebug(debug bool) {
	i.debug = debug
}
