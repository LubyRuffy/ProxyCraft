package certs

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net" // Added for IP address parsing
	"os"
	"strings"
	"time"
)

const (
	caCertFile = "proxycraft-ca.pem"
	caKeyFile  = "proxycraft-ca-key.pem"
	rsaBits    = 2048
)

// Manager handles CA certificate creation and loading.
type Manager struct {
	CACert *x509.Certificate
	CAKey  *rsa.PrivateKey
}

// NewManager creates a new certificate manager.
// It will try to load existing CA cert/key, or generate new ones if not found.
func NewManager() (*Manager, error) {
	m := &Manager{}
	err := m.loadCA()
	if err != nil {
		fmt.Println("CA certificate or key not found, generating new ones...")
		err = m.generateCA()
		if err != nil {
			return nil, fmt.Errorf("failed to generate CA: %w", err)
		}
		fmt.Printf("New CA certificate saved to %s and key to %s\n", caCertFile, caKeyFile)
	} else {
		fmt.Println("Loaded existing CA certificate and key.")
	}
	return m, nil
}

func (m *Manager) generateCA() error {
	privKey, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return fmt.Errorf("failed to generate RSA private key: %w", err)
	}
	m.CAKey = privKey

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"ProxyCraft Generated CA"},
			CommonName:   "ProxyCraft Root CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(10, 0, 0), // Valid for 10 years
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	if err != nil {
		return fmt.Errorf("failed to create certificate: %w", err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return fmt.Errorf("failed to parse created certificate: %w", err)
	}
	m.CACert = cert

	// Save CA certificate to file
	certOut, err := os.Create(caCertFile)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", caCertFile, err)
	}
	defer certOut.Close()
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write CA certificate to %s: %w", caCertFile, err)
	}

	// Save CA private key to file
	keyOut, err := os.OpenFile(caKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", caKeyFile, err)
	}
	defer keyOut.Close()
	privBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return fmt.Errorf("failed to marshal private key: %w", err)
	}
	if err := pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		return fmt.Errorf("failed to write CA key to %s: %w", caKeyFile, err)
	}

	return nil
}

func (m *Manager) loadCA() error {
	certPEM, err := os.ReadFile(caCertFile)
	if err != nil {
		return fmt.Errorf("failed to read CA cert file %s: %w", caCertFile, err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("failed to decode PEM block containing certificate from %s", caCertFile)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA certificate from %s: %w", caCertFile, err)
	}
	m.CACert = cert

	keyPEM, err := os.ReadFile(caKeyFile)
	if err != nil {
		return fmt.Errorf("failed to read CA key file %s: %w", caKeyFile, err)
	}
	block, _ = pem.Decode(keyPEM)
	if block == nil || block.Type != "PRIVATE KEY" {
		return fmt.Errorf("failed to decode PEM block containing private key from %s", caKeyFile)
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse CA private key from %s: %w", caKeyFile, err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("CA key is not an RSA private key in %s", caKeyFile)
	}
	m.CAKey = rsaKey

	return nil
}

// ExportCACert exports the CA certificate to the specified path.
func (m *Manager) ExportCACert(filePath string) error {
	if m.CACert == nil {
		return fmt.Errorf("CA certificate not loaded or generated yet")
	}
	certOut, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to open %s for writing: %w", filePath, err)
	}
	defer certOut.Close()

	derBytes := m.CACert.Raw
	if err := pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		return fmt.Errorf("failed to write CA certificate to %s: %w", filePath, err)
	}
	fmt.Printf("CA certificate exported to %s\n", filePath)
	return nil
}

// GenerateServerCert generates a certificate for the given host, signed by the CA.
func (m *Manager) GenerateServerCert(host string) (*x509.Certificate, *rsa.PrivateKey, error) {
	if m.CACert == nil || m.CAKey == nil {
		return nil, nil, fmt.Errorf("CA certificate or key not loaded")
	}

	// Extract hostname without port if present
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		// If SplitHostPort succeeds, use the hostname part
		hostname = h
	}

	privKey, err := rsa.GenerateKey(rand.Reader, rsaBits)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate server private key for %s: %w", hostname, err)
	}

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate serial number for %s: %w", hostname, err)
	}

	// Add additional SANs for better compatibility
	dnsNames := []string{hostname}

	// Add www. variant if the hostname doesn't already start with www.
	if !strings.HasPrefix(hostname, "www.") {
		wwwVariant := "www." + hostname
		dnsNames = append(dnsNames, wwwVariant)
	}

	// Add wildcard variant for subdomains
	parts := strings.Split(hostname, ".")
	if len(parts) >= 2 {
		// If it's a subdomain like sub.example.com, add *.example.com
		if len(parts) > 2 {
			wildcardDomain := "*." + strings.Join(parts[1:], ".")
			dnsNames = append(dnsNames, wildcardDomain)
		}
		// Also add *.hostname
		wildcardHost := "*." + hostname
		dnsNames = append(dnsNames, wildcardHost)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			CommonName:   hostname, // Important: CN should be the host being impersonated
			Organization: []string{"ProxyCraft MITM Proxy"},
		},
		NotBefore:   time.Now().Add(-1 * time.Hour), // Start 1 hour ago for clock skew
		NotAfter:    time.Now().AddDate(1, 0, 0),    // Valid for 1 year
		KeyUsage:    x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:    dnsNames, // SANs for the host and variants
	}

	// If hostname is an IP address, add it to IPAddresses
	if ip := net.ParseIP(hostname); ip != nil {
		template.IPAddresses = []net.IP{ip}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, m.CACert, &privKey.PublicKey, m.CAKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create server certificate for %s: %w", host, err)
	}

	cert, err := x509.ParseCertificate(derBytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse created server certificate for %s: %w", host, err)
	}

	return cert, privKey, nil
}

// GetCACertPath returns the path to the CA certificate file.
func GetCACertPath() string {
	return caCertFile
}

// LoadCustomCA loads a custom CA certificate and private key from the specified files.
func (m *Manager) LoadCustomCA(certPath, keyPath string) error {
	certPEM, err := os.ReadFile(certPath)
	if err != nil {
		return fmt.Errorf("failed to read custom CA cert file %s: %w", certPath, err)
	}
	block, _ := pem.Decode(certPEM)
	if block == nil || block.Type != "CERTIFICATE" {
		return fmt.Errorf("failed to decode PEM block containing certificate from %s", certPath)
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse custom CA certificate from %s: %w", certPath, err)
	}

	keyPEM, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("failed to read custom CA key file %s: %w", keyPath, err)
	}
	block, _ = pem.Decode(keyPEM)
	if block == nil {
		return fmt.Errorf("failed to decode PEM block containing private key from %s", keyPath)
	}

	// Try to parse the key based on the PEM block type
	var key interface{}
	var rsaKey *rsa.PrivateKey
	var ok bool

	switch block.Type {
	case "PRIVATE KEY":
		key, err = x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse PKCS8 private key from %s: %w", keyPath, err)
		}
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return fmt.Errorf("failed to parse PKCS1 private key from %s: %w", keyPath, err)
		}
	default:
		return fmt.Errorf("unsupported key type %s in %s", block.Type, keyPath)
	}

	// Convert the key to RSA private key
	rsaKey, ok = key.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("custom CA key is not an RSA private key in %s", keyPath)
	}

	// Verify that the key matches the certificate
	if !cert.PublicKey.(*rsa.PublicKey).Equal(rsaKey.Public()) {
		return fmt.Errorf("custom CA certificate and key do not match")
	}

	// Set the certificate and key
	m.CACert = cert
	m.CAKey = rsaKey

	fmt.Printf("Loaded custom CA certificate from %s and key from %s\n", certPath, keyPath)
	return nil
}
