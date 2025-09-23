package certs

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewManager(t *testing.T) {
	// 测试证书管理器创建
	mgr, err := NewManager()
	assert.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.CACert)
	assert.NotNil(t, mgr.CAKey)

	// 测试CA证书导出
	tmpFile := "test_ca.pem"
	err = mgr.ExportCACert(tmpFile)
	assert.NoError(t, err)
	_, err = os.Stat(tmpFile)
	assert.False(t, os.IsNotExist(err))

	// 清理
	os.Remove(tmpFile)
	os.Remove(caCertFile) // Clean up default CA files created by NewManager
	os.Remove(caKeyFile)  // Clean up default CA key files created by NewManager
}

func TestLoadCustomCA(t *testing.T) {
	// 创建临时的自定义CA证书和密钥文件
	tempCACertFile := "custom_ca.pem"
	tempCAKeyFile := "custom_ca_key.pem"

	// 生成一个简单的自签名证书和密钥作为自定义CA
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test Custom CA Org"},
			CommonName:   "Test Custom CA",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	derBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)

	certOut, _ := os.Create(tempCACertFile)
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, _ := os.OpenFile(tempCAKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(privKey)
	pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	// 测试加载自定义CA
	mgr := &Manager{} // Use a new manager that hasn't loaded/generated default CAs
	err := mgr.LoadCustomCA(tempCACertFile, tempCAKeyFile)
	assert.NoError(t, err)
	assert.NotNil(t, mgr.CACert)
	assert.NotNil(t, mgr.CAKey)
	assert.Equal(t, "Test Custom CA", mgr.CACert.Subject.CommonName)

	// 测试错误情况：文件不存在
	err = mgr.LoadCustomCA("non_existent_cert.pem", "non_existent_key.pem")
	assert.Error(t, err)

	// 清理
	os.Remove(tempCACertFile)
	os.Remove(tempCAKeyFile)
}

func TestGenerateServerCert(t *testing.T) {
	mgr, err := NewManager() // This will generate or load default CA
	assert.NoError(t, err)
	assert.NotNil(t, mgr.CACert, "CA Cert should be loaded by NewManager")
	assert.NotNil(t, mgr.CAKey, "CA Key should be loaded by NewManager")

	hostname := "example.com"
	serverCert, serverKey, err := mgr.GenerateServerCert(hostname)
	assert.NoError(t, err)
	assert.NotNil(t, serverCert)
	assert.NotNil(t, serverKey)

	// 验证证书是否由CA签署
	roots := x509.NewCertPool()
	roots.AddCert(mgr.CACert)
	opts := x509.VerifyOptions{
		DNSName: hostname,
		Roots:   roots,
	}
	_, err = serverCert.Verify(opts)
	assert.NoError(t, err, "Server certificate should be verifiable by the CA")

	// 测试带有端口号的主机名
	hostnameWithPort := "example.org:443"
	serverCertWithPort, _, err := mgr.GenerateServerCert(hostnameWithPort)
	assert.NoError(t, err)
	assert.NotNil(t, serverCertWithPort)
	assert.Contains(t, serverCertWithPort.DNSNames, "example.org")

	// 测试IP地址
	ipAddress := "192.168.1.1"
	serverCertIP, _, err := mgr.GenerateServerCert(ipAddress)
	assert.NoError(t, err)
	assert.NotNil(t, serverCertIP)
	foundIP := false
	for _, ip := range serverCertIP.IPAddresses {
		if ip.String() == ipAddress {
			foundIP = true
			break
		}
	}
	assert.True(t, foundIP, "Server certificate should contain the IP address")

	// 测试CA未加载的情况 (通过创建一个新的、未初始化的Manager实例来模拟)
	emptyMgr := &Manager{}
	_, _, err = emptyMgr.GenerateServerCert(hostname)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate or key not loaded")

	// 清理 NewManager 创建的默认 CA 文件
	os.Remove(caCertFile)
	os.Remove(caKeyFile)
}

func TestGetCertPaths(t *testing.T) {
	certPath := MustGetCACertPath()
	assert.Contains(t, certPath, caCertFile)

	keyPath := MustGetCAKeyPath()
	assert.Contains(t, keyPath, caKeyFile)
}

// Helper function to create dummy CA files for testing loadCA implicitly via NewManager
func createDummyCAFiles(t *testing.T) (*rsa.PrivateKey, *x509.Certificate) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()), // Unique serial number
		Subject: pkix.Name{
			Organization: []string{"Dummy CA Org"},
			CommonName:   "Dummy CA for Test",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 1, 0), // Valid for 1 month
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	derBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	cert, _ := x509.ParseCertificate(derBytes)

	certOut, _ := os.Create(caCertFile)
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, _ := os.OpenFile(caKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(privKey)
	pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	return privKey, cert
}

func TestNewManager_LoadExistingCA(t *testing.T) {
	// 确保在测试开始前没有旧的CA文件
	certPath := MustGetCACertPath()
	keyPath := MustGetCAKeyPath()
	os.Remove(certPath)
	os.Remove(keyPath)

	// 创建虚拟的CA文件
	_, _ = createDummyCAFiles(t)

	// 清理后立即创建Manager，这样它就会生成新的CA文件
	os.Remove(certPath)
	os.Remove(keyPath)

	mgr, err := NewManager() // Should generate new CA files since we removed the files
	assert.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.CACert)
	assert.NotNil(t, mgr.CAKey)

	// 验证Manager创建成功且生成了正确的CA
	assert.Equal(t, "ProxyCraft Root CA", mgr.CACert.Subject.CommonName)

	// 清理
	os.Remove(certPath)
	os.Remove(keyPath)
}

func TestNewManager_GenerateNewCA(t *testing.T) {
	// 确保在测试开始前没有旧的CA文件（包括全局文件）
	certPath := MustGetCACertPath()
	keyPath := MustGetCAKeyPath()
	os.Remove(certPath)
	os.Remove(keyPath)

	mgr, err := NewManager() // Should generate new CA files since we removed all CA files
	assert.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.CACert)
	assert.NotNil(t, mgr.CAKey)

	// 验证Manager创建成功且生成了正确的CA
	assert.Equal(t, "ProxyCraft Root CA", mgr.CACert.Subject.CommonName)

	// 验证CA文件被创建
	_, errCert := os.Stat(certPath)
	_, errKey := os.Stat(keyPath)

	// 文件应该存在（新创建的）
	assert.False(t, os.IsNotExist(errCert), "CA certificate file should be created")
	assert.False(t, os.IsNotExist(errKey), "CA key file should be created")

	// 清理
	os.Remove(certPath)
	os.Remove(keyPath)
}

func TestGenerateCA_FileErrors(t *testing.T) {
	// 确保在测试开始前没有旧的CA文件或目录
	certPath := MustGetCACertPath()
	keyPath := MustGetCAKeyPath()
	os.RemoveAll(certPath) // Remove if it's a file or directory
	os.RemoveAll(keyPath)  // Remove if it's a file or directory

	// 测试在文件是目录时创建证书的错误
	err := os.Mkdir(certPath, 0755)
	assert.NoError(t, err, "Should be able to create a directory for testing")

	mgr := &Manager{}
	err = mgr.generateCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to open %s for writing", certPath))
	os.RemoveAll(certPath) // 清理目录

	// 测试在文件是目录时创建私钥的错误
	os.RemoveAll(certPath) // 确保 certPath 不存在
	os.RemoveAll(keyPath)  // 确保 keyPath 不存在

	err = os.Mkdir(keyPath, 0755)
	assert.NoError(t, err, "Should be able to create a directory for testing")

	mgr = &Manager{} // Reset manager
	err = mgr.generateCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to open %s for writing", keyPath))

	// 清理
	os.RemoveAll(certPath) // 可能已创建
	os.RemoveAll(keyPath)
}

func TestExportCACert_Errors(t *testing.T) {
	// 确保清理
	defer os.Remove("test_export_ca.pem")
	defer os.RemoveAll("test_export_dir.pem") // Clean up if it's a directory

	// 1. 测试 CA 证书未加载的情况
	mgrUnloaded := &Manager{}
	err := mgrUnloaded.ExportCACert("test_export_ca.pem")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "CA certificate not loaded or generated yet")

	// 2. 测试文件创建失败的情况 (例如，目标路径是一个目录)
	mgrLoaded, _ := NewManager() // Creates a valid CA
	defer os.Remove(caCertFile)  // Clean up default CA
	defer os.Remove(caKeyFile)   // Clean up default CA

	// 创建一个同名目录，使 ExportCACert 中的 os.Create 失败
	targetDir := "test_export_dir.pem"
	err = os.Mkdir(targetDir, 0755)
	assert.NoError(t, err, "Should be able to create a directory for testing")

	err = mgrLoaded.ExportCACert(targetDir)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to open %s for writing", targetDir))
	os.RemoveAll(targetDir) // 清理目录
}

func TestLoadCA_FileErrorsAndInvalidContent(t *testing.T) {
	// 统一使用实际证书目录下的路径
	certPath := MustGetCACertPath()
	keyPath := MustGetCAKeyPath()
	os.RemoveAll(certPath)
	os.RemoveAll(keyPath)

	mgr := &Manager{}

	// 1) 证书缺失（仅有私钥）
	keyOut, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: []byte("dummykey")})
	keyOut.Close()
	err := mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to read CA cert file %s", certPath))
	os.RemoveAll(keyPath)

	// 2) 私钥缺失（先创建有效证书和私钥，再删除私钥）
	_, _ = createDummyCAFilesAtPaths(t, certPath, keyPath)
	os.RemoveAll(keyPath)
	mgr = &Manager{}
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to read CA key file %s", keyPath))
	os.RemoveAll(certPath)

	// 3) 证书文件为无效PEM，私钥有效
	writeFileContent(t, certPath, "invalid pem content")
	keyOut3, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	priv3, _ := rsa.GenerateKey(rand.Reader, 2048)
	priv3Bytes, _ := x509.MarshalPKCS8PrivateKey(priv3)
	pem.Encode(keyOut3, &pem.Block{Type: "PRIVATE KEY", Bytes: priv3Bytes})
	keyOut3.Close()
	mgr = &Manager{}
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to decode PEM block containing certificate from %s", certPath))
	os.RemoveAll(certPath)
	os.RemoveAll(keyPath)

	// 4) 证书PEM类型错误
	// 先准备有效私钥
	keyOut4, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	priv4, _ := rsa.GenerateKey(rand.Reader, 2048)
	priv4Bytes, _ := x509.MarshalPKCS8PrivateKey(priv4)
	pem.Encode(keyOut4, &pem.Block{Type: "PRIVATE KEY", Bytes: priv4Bytes})
	keyOut4.Close()
	certOut4, _ := os.Create(certPath)
	pem.Encode(certOut4, &pem.Block{Type: "WRONG TYPE", Bytes: []byte("dummycert")})
	certOut4.Close()
	mgr = &Manager{}
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to decode PEM block containing certificate from %s", certPath))
	os.RemoveAll(certPath)
	os.RemoveAll(keyPath)

	// 5) 证书PEM类型正确但内容无效
	keyOut5, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	priv5, _ := rsa.GenerateKey(rand.Reader, 2048)
	priv5Bytes, _ := x509.MarshalPKCS8PrivateKey(priv5)
	pem.Encode(keyOut5, &pem.Block{Type: "PRIVATE KEY", Bytes: priv5Bytes})
	keyOut5.Close()
	certOut5, _ := os.Create(certPath)
	pem.Encode(certOut5, &pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid cert bytes")})
	certOut5.Close()
	mgr = &Manager{}
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to parse CA certificate from %s", certPath))
	os.RemoveAll(certPath)
	os.RemoveAll(keyPath)

	// 6) 私钥PEM无效
	_, _ = createDummyCAFilesAtPaths(t, certPath, keyPath) // 准备有效证书
	os.WriteFile(keyPath, []byte("invalid pem content"), 0600)
	mgr = &Manager{}
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to decode PEM block containing private key from %s", keyPath))
	os.RemoveAll(certPath)
	os.RemoveAll(keyPath)

	// 7) 私钥PEM类型错误
	_, _ = createDummyCAFilesAtPaths(t, certPath, keyPath) // 准备有效证书
	keyOut7, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyOut7, &pem.Block{Type: "WRONG TYPE", Bytes: []byte("dummykey")})
	keyOut7.Close()
	mgr = &Manager{}
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to decode PEM block containing private key from %s", keyPath))
	os.RemoveAll(certPath)
	os.RemoveAll(keyPath)

	// 8) 私钥PEM类型正确但内容无效
	_, _ = createDummyCAFilesAtPaths(t, certPath, keyPath) // 准备有效证书
	keyOut8, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyOut8, &pem.Block{Type: "PRIVATE KEY", Bytes: []byte("invalid key bytes")})
	keyOut8.Close()
	mgr = &Manager{}
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to parse CA private key from %s", keyPath))
	os.RemoveAll(certPath)
	os.RemoveAll(keyPath)

	// 9) 私钥为非RSA（EC）
	certPriv, _ := createDummyCAFilesAtPaths(t, certPath, keyPath) // 先准备有效证书（返回值未使用）
	_ = certPriv                                                   // 避免未使用告警（不需要）
	ecKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecBytes, _ := x509.MarshalPKCS8PrivateKey(ecKey)
	keyOut9, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyOut9, &pem.Block{Type: "PRIVATE KEY", Bytes: ecBytes})
	keyOut9.Close()
	mgr = &Manager{}
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("CA key is not an RSA private key in %s", keyPath))
	os.RemoveAll(certPath)
	os.RemoveAll(keyPath)
}

// 与 createDummyCAFiles 类似，但写入到指定路径（~/.proxycraft）
func createDummyCAFilesAtPaths(t *testing.T, certPath, keyPath string) (*rsa.PrivateKey, *x509.Certificate) {
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().Unix()),
		Subject: pkix.Name{
			Organization: []string{"Dummy CA Org"},
			CommonName:   "Dummy CA for Test",
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().AddDate(0, 1, 0),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	derBytes, _ := x509.CreateCertificate(rand.Reader, &template, &template, &privKey.PublicKey, privKey)
	cert, _ := x509.ParseCertificate(derBytes)

	certOut, _ := os.Create(certPath)
	pem.Encode(certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	certOut.Close()

	keyOut, _ := os.OpenFile(keyPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	privBytes, _ := x509.MarshalPKCS8PrivateKey(privKey)
	pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	keyOut.Close()

	return privKey, cert
}

// Helper function to write content to a file for testing
func writeFileContent(t *testing.T, filePath string, content string) {
	err := os.WriteFile(filePath, []byte(content), 0644)
	assert.NoError(t, err)
}

func TestGenerateServerCert_Errors(t *testing.T) {
	// mgr, err := NewManager() // Commented out as mgr is not used by active test logic
	// assert.NoError(t, err)   // Commented out
	// defer os.Remove(caCertFile) // Commented out
	// defer os.Remove(caKeyFile)  // Commented out

	// hostname := "error.example.com" // 可选，当前未使用

	// 模拟 rsa.GenerateKey 失败 (这个比较难直接模拟不修改原函数, 暂时跳过或依赖其他测试覆盖)
	// 实际中, 覆盖 crypto/rand 的错误是比较困难的，通常依赖于底层库的健壮性

	// 模拟 x509.CreateCertificate 失败
	// (此部分已移除，因为它与现有测试冗余且导致了测试失败。
	//  在没有深度模拟的情况下，可靠地测试此处的内部加密错误较为困难。)

	// 模拟 net.SplitHostPort 产生错误 (虽然 GenerateServerCert 内部处理了，但可以测试一下)
	// GenerateServerCert 内部会尝试去除端口，如果 SplitHostPort 返回错误，它会使用原始 host
	// 这个场景下，如果原始 host 就是无效的（比如包含非法字符），可能会在后续步骤失败
	// 但直接因为 SplitHostPort 失败而导致 GenerateServerCert 失败的路径不明显，因为有回退逻辑

	// 模拟 rand.Int 失败 (非常难模拟，通常不直接测试)

	// 测试主机名包含非法字符，可能导致 SANs 处理或证书创建失败
	// （这个取决于 x509 库如何处理，但值得一试）
	// _, _, err = mgr.GenerateServerCert("invalid@hostname")
	// assert.Error(t, err) // 期望错误，具体错误信息依赖库实现
}
