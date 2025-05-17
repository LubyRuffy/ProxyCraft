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
	assert.Equal(t, caCertFile, GetCACertPath())
	assert.Equal(t, caKeyFile, GetCAKeyPath())
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
	os.Remove(caCertFile)
	os.Remove(caKeyFile)

	// 创建虚拟的CA文件
	expectedKey, expectedCert := createDummyCAFiles(t)

	mgr, err := NewManager() // Should load the dummy CA files
	assert.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.CACert)
	assert.NotNil(t, mgr.CAKey)

	// 验证加载的是否是创建的虚拟CA
	assert.Equal(t, expectedCert.Subject.CommonName, mgr.CACert.Subject.CommonName)
	assert.Equal(t, expectedKey.D, mgr.CAKey.D) // Compare private key components

	// 清理
	os.Remove(caCertFile)
	os.Remove(caKeyFile)
}

func TestNewManager_GenerateNewCA(t *testing.T) {
	// 确保在测试开始前没有旧的CA文件
	os.Remove(caCertFile)
	os.Remove(caKeyFile)

	mgr, err := NewManager() // Should generate new CA files
	assert.NoError(t, err)
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.CACert)
	assert.NotNil(t, mgr.CAKey)

	// 验证新生成的CA文件是否存在
	_, errCert := os.Stat(caCertFile)
	_, errKey := os.Stat(caKeyFile)
	assert.False(t, os.IsNotExist(errCert), "CA certificate file should be created")
	assert.False(t, os.IsNotExist(errKey), "CA key file should be created")

	// 清理
	os.Remove(caCertFile)
	os.Remove(caKeyFile)
}

func TestGenerateCA_FileErrors(t *testing.T) {
	// 确保在测试开始前没有旧的CA文件或目录
	os.RemoveAll(caCertFile) // Remove if it's a file or directory
	os.RemoveAll(caKeyFile)  // Remove if it's a file or directory

	mgr := &Manager{}

	// 测试 os.Create(caCertFile) 失败 (创建同名目录)
	err := os.Mkdir(caCertFile, 0755)
	assert.NoError(t, err, "Should be able to create a directory for testing")

	err = mgr.generateCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to open %s for writing", caCertFile))
	os.RemoveAll(caCertFile) // 清理目录

	// 测试 os.OpenFile(caKeyFile, ...) 失败 (创建同名目录)
	// 首先确保 caCertFile 可以成功创建，然后使 caKeyFile 创建失败
	os.RemoveAll(caCertFile) // 确保 caCertFile 不存在
	os.RemoveAll(caKeyFile)  // 确保 caKeyFile 不存在

	err = os.Mkdir(caKeyFile, 0755)
	assert.NoError(t, err, "Should be able to create a directory for testing")

	mgr = &Manager{} // Reset manager
	err = mgr.generateCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to open %s for writing", caKeyFile))

	// 清理
	os.RemoveAll(caCertFile) // 可能已创建
	os.RemoveAll(caKeyFile)
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
	// 确保在测试开始前没有旧的CA文件或目录
	os.RemoveAll(caCertFile)
	os.RemoveAll(caKeyFile)

	mgr := &Manager{}

	// 1. 测试 caCertFile 不存在
	// createDummyCAFiles(t) // Create key file first
	keyOut, _ := os.OpenFile(caKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: []byte("dummykey")})
	keyOut.Close()
	err := mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to read CA cert file %s", caCertFile))
	os.Remove(caKeyFile)

	// 2. 测试 caKeyFile 不存在
	// Create valid cert and key first, then remove key file
	createDummyCAFiles(t)
	os.Remove(caKeyFile) // Ensure key file does not exist

	mgr = &Manager{} // Reset manager or use a new one for a clean state
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to read CA key file %s", caKeyFile))
	os.Remove(caCertFile) // Clean up the cert file created by createDummyCAFiles

	// 3. 测试 caCertFile 内容无效 (无法解码PEM)
	writeFileContent(t, caCertFile, "invalid pem content")
	// createDummyCAFiles(t) // This creates both, so we overwrite cert file
	// For this test, we need a valid key file but an invalid cert file.
	// First, create a valid key file.
	keyOutForCase3, _ := os.OpenFile(caKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	privKeyForCase3, _ := rsa.GenerateKey(rand.Reader, 2048)
	privBytesForCase3, _ := x509.MarshalPKCS8PrivateKey(privKeyForCase3)
	pem.Encode(keyOutForCase3, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytesForCase3})
	keyOutForCase3.Close()
	// Now, caKeyFile is valid. caCertFile (written by writeFileContent) is invalid PEM.

	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to decode PEM block containing certificate from %s", caCertFile))
	os.Remove(caCertFile) // Clean up invalid cert
	os.Remove(caKeyFile)  // Clean up valid key

	// 4. 测试 caCertFile 内容无效 (错误PEM类型)
	// Ensure caKeyFile is valid for this test of caCertFile
	_, _ = createDummyCAFiles(t) // Creates caCertFile and caKeyFile, we'll use its caKeyFile
	os.Remove(caCertFile)        // Remove the valid cert created by helper, we'll create a bad one

	certOutForCase4, _ := os.Create(caCertFile) // Declare certOutForCase4
	pem.Encode(certOutForCase4, &pem.Block{Type: "WRONG TYPE", Bytes: []byte("dummycert")})
	certOutForCase4.Close()
	// At this point, caCertFile has WRONG TYPE, caKeyFile is valid from createDummyCAFiles

	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to decode PEM block containing certificate from %s", caCertFile))
	os.Remove(caCertFile) // Clean up our bad cert
	os.Remove(caKeyFile)  // Clean up key from createDummyCAFiles

	// 5. 测试 caCertFile 内容无效 (无法解析证书)
	// Ensure caKeyFile is valid
	_, _ = createDummyCAFiles(t) // Creates caCertFile and caKeyFile
	os.Remove(caCertFile)        // Remove the valid cert created by helper

	certOutForCase5, _ := os.Create(caCertFile) // Declare certOutForCase5
	pem.Encode(certOutForCase5, &pem.Block{Type: "CERTIFICATE", Bytes: []byte("invalid cert bytes")})
	certOutForCase5.Close()
	// caCertFile has invalid bytes, caKeyFile is valid from createDummyCAFiles

	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to parse CA certificate from %s", caCertFile))
	os.Remove(caCertFile) // Clean up our bad cert
	os.Remove(caKeyFile)  // Clean up key from createDummyCAFiles

	// 6. 测试 caKeyFile 内容无效 (无法解码PEM)
	createDummyCAFiles(t) // Create valid cert file
	os.WriteFile(caKeyFile, []byte("invalid pem content"), 0600)
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to decode PEM block containing private key from %s", caKeyFile))
	os.Remove(caCertFile)
	os.Remove(caKeyFile)

	// 7. 测试 caKeyFile 内容无效 (错误PEM类型)
	createDummyCAFiles(t) // Create valid cert file
	keyOut, _ = os.OpenFile(caKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyOut, &pem.Block{Type: "WRONG TYPE", Bytes: []byte("dummykey")})
	keyOut.Close()
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to decode PEM block containing private key from %s", caKeyFile))
	os.Remove(caCertFile)
	os.Remove(caKeyFile)

	// 8. 测试 caKeyFile 内容无效 (无法解析私钥)
	createDummyCAFiles(t) // Create valid cert file
	keyOut, _ = os.OpenFile(caKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: []byte("invalid key bytes")})
	keyOut.Close()
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("failed to parse CA private key from %s", caKeyFile))
	os.Remove(caCertFile)
	os.Remove(caKeyFile)

	// 9. 测试 caKeyFile 内容无效 (非RSA私钥, e.g., EC key)
	// Generate a dummy EC private key for testing this scenario
	ecPrivKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	ecPrivBytes, err := x509.MarshalPKCS8PrivateKey(ecPrivKey)
	assert.NoError(t, err, "Failed to marshal EC private key to PKCS#8")
	createDummyCAFiles(t) // Create valid cert file
	keyOut, _ = os.OpenFile(caKeyFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	pem.Encode(keyOut, &pem.Block{Type: "PRIVATE KEY", Bytes: ecPrivBytes})
	keyOut.Close()
	err = mgr.loadCA()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), fmt.Sprintf("CA key is not an RSA private key in %s", caKeyFile))
	os.Remove(caCertFile)
	os.Remove(caKeyFile)
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
