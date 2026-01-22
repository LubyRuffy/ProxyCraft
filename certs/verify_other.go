//go:build !darwin

package certs

import (
	"crypto/rsa"
	"crypto/x509"
	"fmt"
)

func verifySystemTrustWithCert(_ []byte, _ *rsa.PrivateKey, _ *x509.Certificate) error {
	return fmt.Errorf("system trust verification not supported on this platform")
}
