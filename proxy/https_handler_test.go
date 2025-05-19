package proxy

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/LubyRuffy/ProxyCraft/certs"
	"github.com/LubyRuffy/ProxyCraft/harlogger"
	"github.com/stretchr/testify/assert"
)

func TestHTTPSHandler(t *testing.T) {
	// 创建测试用的后端服务器
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello from backend"))
	})
	backendServer := httptest.NewServer(backend)
	defer backendServer.Close()

	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建代理服务器 - 使用随机端口避免冲突
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	proxyAddr := listener.Addr().String()
	listener.Close() // 关闭监听器，让服务器可以使用这个端口

	// 创建代理服务器
	server := NewServer(
		proxyAddr,
		certMgr,
		false,
		harLog,
		true, // 启用MITM模式
		nil,  // 不使用上游代理
		false,
	)
	assert.NotNil(t, server)

	// 启动代理服务器
	go server.Start()
	time.Sleep(100 * time.Millisecond) // 等待服务器启动

	// 创建测试客户端
	proxyURL, _ := url.Parse("http://" + proxyAddr)
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:           http.ProxyURL(proxyURL),
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	// 发送测试请求到测试服务器而不是真实的example.com
	testServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello from HTTPS server"))
	}))
	defer testServer.Close()

	// 发送测试请求
	resp, err := client.Get(testServer.URL)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()
	assert.Contains(t, string(body), "Hello from HTTPS server")
}

func TestServerHTTPSHandlersWithDifferentModes(t *testing.T) {
	// 创建后端HTTPS服务器
	backendServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
		w.Write([]byte("Hello from HTTPS backend"))
	}))
	defer backendServer.Close()

	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 测试不同的代理服务器模式
	testCases := []struct {
		name     string
		mitmMode bool
		expectOK bool
	}{
		{
			name:     "MITM模式-允许拦截HTTPS",
			mitmMode: true,
			expectOK: true,
		},
		{
			name:     "非MITM模式-隧道传输HTTPS",
			mitmMode: false,
			expectOK: true, // 隧道模式也应该工作
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建代理服务器 - 使用随机端口避免冲突
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			assert.NoError(t, err)
			proxyAddr := listener.Addr().String()
			listener.Close() // 关闭监听器，让服务器可以使用这个端口

			// 创建代理服务器
			server := NewServer(
				proxyAddr,
				certMgr,
				true, // 启用详细日志
				harLog,
				tc.mitmMode,
				nil, // 不使用上游代理
				false,
			)
			assert.NotNil(t, server)

			// 启动代理服务器
			go server.Start()
			time.Sleep(100 * time.Millisecond) // 等待服务器启动

			// 创建测试客户端
			proxyURL, _ := url.Parse("http://" + proxyAddr)
			client := &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
					TLSClientConfig: &tls.Config{
						InsecureSkipVerify: true, // 忽略证书验证，因为我们使用的是自签名证书
					},
				},
			}

			// 发送测试请求
			resp, err := client.Get(backendServer.URL)

			if tc.expectOK {
				assert.NoError(t, err)
				if resp != nil {
					defer resp.Body.Close()
					assert.Equal(t, 200, resp.StatusCode)

					// 读取响应体
					body, err := io.ReadAll(resp.Body)
					assert.NoError(t, err)
					assert.Contains(t, string(body), "Hello from HTTPS backend")
				}
			}
		})
	}
}

type notHijackableResponseWriter struct {
	http.ResponseWriter
}

func TestHTTPSHandleHijackError(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建服务器
	server := NewServer(
		"127.0.0.1:0",
		certMgr,
		true,
		harLog,
		false,
		nil,
		false,
	)

	// 创建一个不支持劫持的ResponseWriter
	nonHijackableWriter := &notHijackableResponseWriter{
		ResponseWriter: httptest.NewRecorder(),
	}

	// 创建一个CONNECT请求
	req, _ := http.NewRequest("CONNECT", "https://example.com:443", nil)

	// 直接调用handleHTTPS方法，预期会返回错误
	server.handleHTTPS(nonHijackableWriter, req)

	// 由于ResponseWriter不支持劫持，预期handleHTTPS会写入500错误
	recorder := nonHijackableWriter.ResponseWriter.(*httptest.ResponseRecorder)
	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
}

// 测试使用上游代理的情况
func TestHTTPSWithUpstreamProxy(t *testing.T) {
	// 创建上游代理服务器
	upstreamProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "CONNECT" {
			// 处理 CONNECT 请求，模拟隧道建立成功
			w.WriteHeader(http.StatusOK)
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("上游代理不支持 Hijacking")
			}
			clientConn, _, err := hijacker.Hijack()
			if err != nil {
				t.Fatalf("无法劫持连接: %v", err)
			}
			defer clientConn.Close()

			// 回复模拟的数据
			clientConn.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
			// 在实际情况下，这里会进行数据传输，但测试中仅模拟连接建立
		} else {
			// 处理非 CONNECT 请求
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello from upstream proxy"))
		}
	}))
	defer upstreamProxy.Close()

	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建代理服务器 - 使用随机端口避免冲突
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	proxyAddr := listener.Addr().String()
	listener.Close() // 关闭监听器，让服务器可以使用这个端口

	// 设置上游代理 URL
	upstreamProxyURL, _ := url.Parse(upstreamProxy.URL)

	// 创建代理服务器
	server := NewServer(
		proxyAddr,
		certMgr,
		true, // 启用详细日志
		harLog,
		false, // 不启用 MITM 模式
		upstreamProxyURL,
		false,
	)
	assert.NotNil(t, server)

	// 启动代理服务器
	go server.Start()
	time.Sleep(100 * time.Millisecond) // 等待服务器启动

	// 注意：实际测试上游代理需要创建真实连接，这里我们只验证代理服务器创建成功
}

// 测试 HTTPS MITM 模式下证书生成失败的情况
func TestHTTPSMITMCertificateError(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建一个非MITM的代理服务器
	server := &Server{
		Addr:        "127.0.0.1:0",
		CertManager: certMgr,
		Verbose:     true,
		HarLogger:   harLog,
		EnableMITM:  true, // 启用MITM模式
	}

	// 创建一个带有无效主机名的 CONNECT 请求，这将导致证书生成失败
	req, _ := http.NewRequest("CONNECT", "https://invalid_host_for_cert:443", nil)
	w := httptest.NewRecorder()

	// 注意：由于 ResponseWriter 不支持 Hijacker 接口，这将导致错误
	server.handleHTTPS(w, req)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// 测试 TLS 握手失败的情况
func TestHTTPSMITMTLSHandshakeFailure(t *testing.T) {
	// 这个测试需要更高级的网络模拟，这里我们只测试最基本的情况
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建代理服务器
	server := NewServer(
		"127.0.0.1:0",
		certMgr,
		true,
		harLog,
		true, // 启用 MITM 模式
		nil,
		true, // 启用流量输出
	)

	// 创建一个 CONNECT 请求
	req, _ := http.NewRequest("CONNECT", "https://example.com:443", nil)
	w := httptest.NewRecorder()

	// 注意：这个测试无法完全模拟 TLS 握手失败的情况，仅为了覆盖部分代码
	server.handleHTTPS(w, req)
}

// mockCertManager 是一个模拟的证书管理器
type mockCertManager struct {
	generateError bool
	caCert        *x509.Certificate
	caKey         *rsa.PrivateKey
}

// GenerateCertificate 生成证书
func (m *mockCertManager) GenerateCertificate(hostname string) (*tls.Certificate, error) {
	if m.generateError {
		return nil, assert.AnError
	}

	// 生成自签名证书
	privKey, _ := rsa.GenerateKey(rand.Reader, 2048)

	template := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: hostname,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour),
		KeyUsage:  x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
	}

	derBytes, _ := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
	cert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  privKey,
	}

	return &cert, nil
}

// GetCertificate 实现 tls.Config 的 GetCertificate 方法
func (m *mockCertManager) GetCertificate(clientHello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	return m.GenerateCertificate(clientHello.ServerName)
}

// GetServerTLSConfig 返回服务器 TLS 配置
func (m *mockCertManager) GetServerTLSConfig() *tls.Config {
	return &tls.Config{
		GetCertificate: m.GetCertificate,
	}
}

// GetCACertPEM 返回 CA 证书的 PEM 编码
func (m *mockCertManager) GetCACertPEM() []byte {
	// 创建一个简单的自签名证书作为 CA 证书
	if m.caCert == nil {
		privKey, _ := rsa.GenerateKey(rand.Reader, 2048)
		m.caKey = privKey

		template := &x509.Certificate{
			SerialNumber: big.NewInt(1),
			Subject: pkix.Name{
				CommonName: "Mock CA",
			},
			NotBefore:             time.Now(),
			NotAfter:              time.Now().Add(time.Hour),
			KeyUsage:              x509.KeyUsageCertSign,
			BasicConstraintsValid: true,
			IsCA:                  true,
		}

		derBytes, _ := x509.CreateCertificate(rand.Reader, template, template, &privKey.PublicKey, privKey)
		cert, _ := x509.ParseCertificate(derBytes)
		m.caCert = cert
	}

	certBytes := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: m.caCert.Raw})
	return certBytes
}

// GetCAKeyPEM 返回 CA 私钥的 PEM 编码
func (m *mockCertManager) GetCAKeyPEM() []byte {
	if m.caKey == nil {
		// 如果没有 caKey，先调用 GetCACertPEM 生成
		m.GetCACertPEM()
	}

	privBytes, _ := x509.MarshalPKCS8PrivateKey(m.caKey)
	keyBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	return keyBytes
}
