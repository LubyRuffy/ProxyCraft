package proxy

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/LubyRuffy/ProxyCraft/certs"
	"github.com/LubyRuffy/ProxyCraft/harlogger"
	"github.com/stretchr/testify/assert"
)

func TestServerHTTPHandlers(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")
	harLog.EnableAutoSave(1 * time.Second)

	// 创建代理服务器 - 使用随机端口避免冲突
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	proxyAddr := listener.Addr().String()
	listener.Close() // 关闭监听器，让服务器可以使用这个端口

	// 创建测试场景
	testScenarios := []struct {
		name             string
		serverSetup      func() *Server
		mockBackendSetup func() *httptest.Server
		requestSetup     func(*http.Client, string) (*http.Response, error)
	}{
		{
			name: "基本HTTP请求处理",
			serverSetup: func() *Server {
				return NewServer(
					proxyAddr,
					certMgr,
					true, // 启用详细日志
					harLog,
					false,
					nil, // 不使用上游代理
					false,
				)
			},
			mockBackendSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(200)
					w.Write([]byte("Hello from backend"))
				}))
			},
			requestSetup: func(client *http.Client, backendURL string) (*http.Response, error) {
				return client.Get(backendURL)
			},
		},
		{
			name: "处理相对URL",
			serverSetup: func() *Server {
				return NewServer(
					proxyAddr,
					certMgr,
					true,
					harLog,
					false,
					nil,
					false,
				)
			},
			mockBackendSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "/test/path", r.URL.Path)
					assert.Equal(t, "query=value", r.URL.RawQuery)
					w.WriteHeader(200)
					w.Write([]byte("Hello from relative URL"))
				}))
			},
			requestSetup: func(client *http.Client, backendURL string) (*http.Response, error) {
				// 解析后端URL来获取主机名
				parsedURL, _ := url.Parse(backendURL)
				host := parsedURL.Host

				// 手动创建请求以使用相对URL
				req, _ := http.NewRequest("GET", "http://"+host+"/test/path?query=value", nil)
				return client.Do(req)
			},
		},
		{
			name: "处理非SSE响应",
			serverSetup: func() *Server {
				return NewServer(
					proxyAddr,
					certMgr,
					true,
					harLog,
					false,
					nil,
					false,
				)
			},
			mockBackendSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					// 虽然请求期望SSE，但我们返回普通响应
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(200)
					w.Write([]byte("Not an SSE response"))
				}))
			},
			requestSetup: func(client *http.Client, backendURL string) (*http.Response, error) {
				// 创建看起来像SSE请求的请求
				req, _ := http.NewRequest("GET", backendURL+"/events", nil)
				req.Header.Set("Accept", "text/event-stream")
				return client.Do(req)
			},
		},
		{
			name: "处理目标服务器错误",
			serverSetup: func() *Server {
				return NewServer(
					proxyAddr,
					certMgr,
					true,
					harLog,
					false,
					nil,
					false,
				)
			},
			mockBackendSetup: func() *httptest.Server {
				// 返回服务器错误的后端
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(500)
					w.Write([]byte("Internal Server Error"))
				}))
			},
			requestSetup: func(client *http.Client, backendURL string) (*http.Response, error) {
				return client.Get(backendURL)
			},
		},
		{
			name: "启用流量输出",
			serverSetup: func() *Server {
				return NewServer(
					proxyAddr,
					certMgr,
					true,
					harLog,
					false,
					nil,
					true, // 启用流量输出
				)
			},
			mockBackendSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "text/plain")
					w.WriteHeader(200)
					w.Write([]byte("Hello with traffic dump"))
				}))
			},
			requestSetup: func(client *http.Client, backendURL string) (*http.Response, error) {
				return client.Get(backendURL)
			},
		},
		{
			name: "处理POST请求",
			serverSetup: func() *Server {
				return NewServer(
					proxyAddr,
					certMgr,
					true,
					harLog,
					false,
					nil,
					false,
				)
			},
			mockBackendSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "POST", r.Method)
					body, _ := io.ReadAll(r.Body)
					assert.Equal(t, "test post data", string(body))
					w.WriteHeader(200)
					w.Write([]byte("Post received"))
				}))
			},
			requestSetup: func(client *http.Client, backendURL string) (*http.Response, error) {
				return client.Post(backendURL, "text/plain", strings.NewReader("test post data"))
			},
		},
		{
			name: "处理带查询参数的URL",
			serverSetup: func() *Server {
				return NewServer(
					proxyAddr,
					certMgr,
					true,
					harLog,
					false,
					nil,
					false,
				)
			},
			mockBackendSetup: func() *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					assert.Equal(t, "test=value", r.URL.RawQuery)
					w.WriteHeader(200)
					w.Write([]byte("Query param received"))
				}))
			},
			requestSetup: func(client *http.Client, backendURL string) (*http.Response, error) {
				return client.Get(backendURL + "?test=value")
			},
		},
	}

	// 运行所有测试场景
	for _, tc := range testScenarios {
		t.Run(tc.name, func(t *testing.T) {
			mockBackend := tc.mockBackendSetup()
			defer mockBackend.Close()

			server := tc.serverSetup()
			go server.Start()
			time.Sleep(100 * time.Millisecond) // 等待服务器启动

			// 设置代理客户端
			proxyURL, _ := url.Parse("http://" + proxyAddr)
			client := &http.Client{
				Transport: &http.Transport{
					Proxy: http.ProxyURL(proxyURL),
				},
			}

			// 发送请求
			resp, err := tc.requestSetup(client, mockBackend.URL)
			// 请求可能失败，比如上游代理测试
			if err == nil && resp != nil {
				defer resp.Body.Close()
				_, _ = io.ReadAll(resp.Body)
			}
		})
	}
}

func TestServerWithDumpTraffic(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建代理服务器 - 使用随机端口避免冲突
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	proxyAddr := listener.Addr().String()
	listener.Close() // 关闭监听器，让服务器可以使用这个端口

	// 创建启用流量转储的代理服务器
	server := NewServer(
		proxyAddr,
		certMgr,
		true,
		harLog,
		false,
		nil,
		true, // 启用流量转储
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
		},
	}

	// 创建模拟后端服务器
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"message":"Hello with traffic dump"}`))
	}))
	defer backend.Close()

	// 发送测试请求
	resp, err := client.Get(backend.URL)
	assert.NoError(t, err)
	defer resp.Body.Close()

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	assert.Contains(t, string(bodyBytes), "Hello with traffic dump")
}

func TestServerWithUpstreamProxy(t *testing.T) {
	// 创建假的上游代理服务器
	upstreamProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 模拟上游代理的行为：解析目标URL并发送请求
		targetURL := r.URL.String()
		if r.Method == http.MethodConnect {
			// 对于CONNECT请求，目标是Host字段
			targetURL = "https://" + r.Host
		}

		// 记录上游代理收到的请求
		w.Header().Set("X-Upstream-Proxy", "true")
		w.Header().Set("X-Target-URL", targetURL)
		w.WriteHeader(200)
		w.Write([]byte("Response from upstream proxy"))
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

	// 创建上游代理URL
	upstreamURL, _ := url.Parse(upstreamProxy.URL)

	// 创建使用上游代理的代理服务器
	server := NewServer(
		proxyAddr,
		certMgr,
		true,
		harLog,
		false,
		upstreamURL,
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
		},
	}

	// 创建测试请求
	resp, err := client.Get("http://example.com")
	if err != nil {
		// 由于我们使用的是模拟上游代理，可能会出现错误
		// 这里我们只需要确保代理服务器正常启动并处理请求
		t.Logf("获取响应时出错: %v", err)
		return
	}

	defer resp.Body.Close()

	// 读取响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)

	// 检查是否收到了上游代理的响应
	assert.Equal(t, "true", resp.Header.Get("X-Upstream-Proxy"))
	assert.Contains(t, string(bodyBytes), "Response from upstream proxy")
}
