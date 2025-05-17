package proxy

import (
	"crypto/tls"
	"io"
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
