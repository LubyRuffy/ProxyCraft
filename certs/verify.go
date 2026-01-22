package certs

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"runtime"
)

// VerifySystemTrust starts a local TLS server signed by the CA and verifies
// that the system trust store accepts it without warnings.
func VerifySystemTrust(m *Manager) error {
	if m == nil {
		return fmt.Errorf("certificate manager is nil")
	}
	if m.CACert == nil {
		return fmt.Errorf("CA certificate is not loaded")
	}

	if runtime.GOOS == "darwin" {
		certPEM, err := encodeCertPEM(m.CACert)
		if err != nil {
			return fmt.Errorf("failed to encode CA certificate: %w", err)
		}
		if err := verifySystemTrustWithCert(certPEM, nil, m.CACert); err != nil {
			return fmt.Errorf("system trust verification failed: %w", err)
		}
		return nil
	}

	cert, key, err := m.GenerateServerCert("localhost")
	if err != nil {
		return fmt.Errorf("failed to generate server certificate: %w", err)
	}

	certPEM, err := encodeCertPEM(cert)
	if err != nil {
		return fmt.Errorf("failed to encode server certificate: %w", err)
	}

	if err := verifySystemTrustWithCert(certPEM, key, cert); err != nil {
		return fmt.Errorf("system trust verification failed: %w", err)
	}

	return nil
}

func encodeCertPEM(cert *x509.Certificate) ([]byte, error) {
	if cert == nil {
		return nil, fmt.Errorf("certificate is nil")
	}
	block := &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}
	return pem.EncodeToMemory(block), nil
}
