package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"time"
)

var (
	IssuerName = "ProxyCraft Root CA"         // Issuer Name
	OrgName    = "ProxyCraft Inc."            // Organization Name
	NotAfter   = time.Now().AddDate(10, 0, 0) // 10 years
)

const (
	caCertFile = "proxycraft-ca.pem"     // CA certificate file name
	caKeyFile  = "proxycraft-ca-key.pem" // CA private key file name
	rsaBits    = 2048                    // RSA bits
)

// mustGetCertDir returns the directory where certificates are stored (~/.proxycraft).
// It creates the directory if it doesn't exist.
func mustGetCertDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("could not get user home directory: %v", err)
	}
	certDir := filepath.Join(home, ".proxycraft")
	if err := os.MkdirAll(certDir, 0755); err != nil {
		log.Fatalf("could not create cert directory %s: %v", certDir, err)
	}
	return certDir
}

// generateCA generates a CA certificate and private key.
func generateCA(issuerName string, orgName string, notAfter time.Time) (*x509.Certificate, *rsa.PrivateKey, error) {
	privKey, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate RSA private key: %w", err)
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{orgName},
			CommonName:   issuerName,
		},
		NotBefore:             time.Now().AddDate(0, 0, -1), // Start 1 day ago for clock skew
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse created certificate: %w", err)
	}

	return cert, privKey, nil
}

// GenerateToFile generates a CA certificate and private key to the specified file path.
func generateToFile(certPath, keyPath string, issuerName string, orgName string, notAfter time.Time) error {
	cert, key, err := generateCA(issuerName, orgName, notAfter)
	if err != nil {
		return fmt.Errorf("failed to generate CA: %w", err)
	}

	// Save CA certificate to file
	certOut, err := os.Create(certPath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", certPath, err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: cert.Raw}); err != nil {
		return fmt.Errorf("failed to write CA certificate to %s: %w", certPath, err)
	}

	// Save CA private key to file
	keyOut, err := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", keyPath, err)
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write CA key to %s: %w", keyPath, err)
	}

	return nil
}

func IsInstalled() bool {
	installed, err := isInstalled()
	if err != nil {
		log.Fatalf("failed to check if certificate is installed: %v", err)
	}
	return installed
}

// Install installs the CA certificate to the system keychain.
func Install() error {
	// 先检查证书是否已经安装
	installed := IsInstalled()
	if installed {
		fmt.Println("CA certificate is already installed in the system keychain. Skipping installation.")
		return nil
	}

	// 生成证书
	certDir := mustGetCertDir()
	certPath := filepath.Join(certDir, caCertFile)
	keyPath := filepath.Join(certDir, caKeyFile)
	needToGenerate := false
	if _, err := os.Stat(certPath); os.IsNotExist(err) {
		needToGenerate = true
	} else if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		needToGenerate = true
	}
	if needToGenerate {
		err := generateToFile(certPath, keyPath, IssuerName, OrgName, NotAfter)
		if err != nil {
			return fmt.Errorf("failed to generate CA: %w", err)
		}
	}

	// 安装证书
	err := install()
	if err != nil {
		return fmt.Errorf("failed to install certificate: %w", err)
	}

	fmt.Println("Certificate installed successfully.")
	return nil
}

// Uninstall uninstalls the CA certificate from the system keychain.
func Uninstall() error {
	// 先检查证书是否已经安装
	installed, err := isInstalled()
	if err != nil {
		return fmt.Errorf("failed to check if certificate is installed: %w", err)
	}
	if !installed {
		fmt.Println("CA certificate is not installed in the system keychain. Skipping uninstallation.")
		return nil
	}

	// 卸载证书
	err = uninstall()
	if err != nil {
		return fmt.Errorf("failed to uninstall certificate: %w", err)
	}

	fmt.Println("Certificate uninstalled successfully.")
	return nil
}
