//go:build darwin

package certs

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

func verifySystemTrustWithCert(certPEM []byte, _ *rsa.PrivateKey, _ *x509.Certificate) error {
	if len(certPEM) == 0 {
		return fmt.Errorf("certificate data is empty")
	}

	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("failed to decode certificate PEM")
	}

	localCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	keychains := []string{systemKeychain}
	if currentUser, err := user.Current(); err == nil {
		loginKeychain := filepath.Join(currentUser.HomeDir, "Library/Keychains/login.keychain-db")
		keychains = append(keychains, loginKeychain)
	}

	var queryErrors []string
	found := false
	for _, keychain := range keychains {
		cmd := exec.Command("/usr/bin/security", "find-certificate", "-c", IssuerName, "-p", keychain)
		output, err := cmd.CombinedOutput()
		if err != nil {
			msg := strings.TrimSpace(string(output))
			if msg == "" {
				msg = err.Error()
			}
			queryErrors = append(queryErrors, fmt.Sprintf("%s: %s", keychain, msg))
			continue
		}
		if countCerts(output) > 0 {
			found = true
		}
		if certsMatchPEM(output, localCert.Raw) {
			return nil
		}
	}

	if len(queryErrors) == len(keychains) {
		return fmt.Errorf("failed to query keychains: %s", strings.Join(queryErrors, "; "))
	}
	if found {
		return fmt.Errorf("certificate found but does not match the local CA")
	}

	return fmt.Errorf("certificate not found in system or login keychain")
}

func certsMatchPEM(pemData []byte, raw []byte) bool {
	remaining := pemData
	for len(remaining) > 0 {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type != "CERTIFICATE" {
			continue
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			continue
		}
		if bytes.Equal(cert.Raw, raw) {
			return true
		}
	}
	return false
}

func countCerts(pemData []byte) int {
	count := 0
	remaining := pemData
	for len(remaining) > 0 {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			break
		}
		if block.Type == "CERTIFICATE" {
			count++
		}
	}
	return count
}
