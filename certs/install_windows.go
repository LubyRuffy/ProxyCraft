//go:build windows

package certs

import (
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

func isInstalled() (bool, error) {
	output, err := runCertutil("-store", "root", IssuerName)
	if err != nil {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "no certificate matches") || strings.Contains(lower, "cannot find object") {
			return false, nil
		}
		return false, err
	}
	return strings.Contains(output, IssuerName), nil
}

func install() error {
	certPath := filepath.Clean(MustGetCACertPath())
	output, err := runCertutil("-addstore", "-f", "root", certPath)
	if err != nil {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "access is denied") {
			return fmt.Errorf("failed to install certificate: access denied. Please run ProxyCraft as Administrator")
		}
		return err
	}
	return nil
}

func uninstall() error {
	output, err := runCertutil("-delstore", "root", IssuerName)
	if err != nil {
		lower := strings.ToLower(output)
		if strings.Contains(lower, "cannot find object") || strings.Contains(lower, "no certificate matches") {
			return nil
		}
		return err
	}
	return nil
}

func runCertutil(args ...string) (string, error) {
	cmd := exec.Command("certutil", args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.CombinedOutput()
	trimmed := strings.TrimSpace(string(output))
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return trimmed, fmt.Errorf("certutil is not available on this system: %w", err)
		}
		return trimmed, fmt.Errorf("certutil %s failed: %w (output: %s)", strings.Join(args, " "), err, trimmed)
	}
	return trimmed, nil
}
