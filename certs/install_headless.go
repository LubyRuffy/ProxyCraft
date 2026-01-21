//go:build !darwin && !windows && !linux

package certs

import (
	"fmt"
)

// isInstalled checks if the CA certificate is already installed in the system trust store.
func isInstalled() (bool, error) {
	return false, nil // Not implemented for this platform
}

// InstallCerts installs the CA certificate to the system trust store.
// It requires sudo privileges. If the certificate is already installed, it will skip the installation.
func install() error {
	return fmt.Errorf("automatic certificate installation is not supported on this platform")
}

// uninstall uninstalls the CA certificate from the system trust store.
// It requires sudo privileges.
func uninstall() error {
	return fmt.Errorf("automatic certificate uninstallation is not supported on this platform")
}