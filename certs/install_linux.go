//go:build linux

package certs

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type linuxTrustTarget struct {
	path    string
	refresh []string
}

var linuxTrustTargets = []linuxTrustTarget{
	{path: filepath.Join("/usr/local/share/ca-certificates", "proxycraft-root-ca.crt"), refresh: []string{"update-ca-certificates"}},
	{path: filepath.Join("/etc/pki/ca-trust/source/anchors", "proxycraft-root-ca.pem"), refresh: []string{"update-ca-trust", "extract"}},
}

func isInstalled() (bool, error) {
	for _, target := range linuxTrustTargets {
		if fileExists(target.path) {
			return true, nil
		}
	}
	return false, nil
}

func install() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("installing the root CA requires root privileges; please rerun with sudo")
	}

	certPath := MustGetCACertPath()
	var attempted bool
	var errs []error

	for _, target := range linuxTrustTargets {
		if len(target.refresh) == 0 || !commandExists(target.refresh[0]) {
			continue
		}
		attempted = true

		if err := copyFile(certPath, target.path, 0644); err != nil {
			errs = append(errs, fmt.Errorf("copy to %s failed: %w", target.path, err))
			continue
		}

		if err := runCommand(target.refresh[0], target.refresh[1:]...); err != nil {
			errs = append(errs, err)
			continue
		}

		return nil
	}

	if !attempted {
		return fmt.Errorf("no supported CA trust manager found (looked for update-ca-certificates/update-ca-trust)")
	}

	return errors.Join(errs...)
}

func installForce() error {
	return install()
}

func uninstall() error {
	if os.Geteuid() != 0 {
		return fmt.Errorf("removing the root CA requires root privileges; please rerun with sudo")
	}

	var attempted bool
	var errs []error

	for _, target := range linuxTrustTargets {
		if len(target.refresh) == 0 || !commandExists(target.refresh[0]) {
			continue
		}

		attempted = true

		if err := os.Remove(target.path); err != nil && !os.IsNotExist(err) {
			errs = append(errs, fmt.Errorf("remove %s failed: %w", target.path, err))
			continue
		}

		if err := runCommand(target.refresh[0], target.refresh[1:]...); err != nil {
			errs = append(errs, err)
			continue
		}
	}

	if !attempted {
		return fmt.Errorf("no supported CA trust manager found (looked for update-ca-certificates/update-ca-trust)")
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}

	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}

	return out.Chmod(perm)
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	trimmed := strings.TrimSpace(string(output))
	joinedArgs := strings.Join(args, " ")
	if trimmed != "" {
		return fmt.Errorf("%s %s failed: %w (output: %s)", name, joinedArgs, err, trimmed)
	}
	return fmt.Errorf("%s %s failed: %w", name, joinedArgs, err)
}
