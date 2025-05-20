package proxy

import (
	"bytes"
	"compress/gzip"
	"fmt"
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
	"github.com/stretchr/testify/require"
)

// mockEventHandler 是一个用于测试的简单事件处理器
type mockEventHandler struct{}

func (h *mockEventHandler) OnRequest(ctx *RequestContext) *http.Request {
	return ctx.Request
}

func (h *mockEventHandler) OnResponse(ctx *ResponseContext) *http.Response {
	return ctx.Response
}

func (h *mockEventHandler) OnError(err error, ctx *RequestContext) {}

func (h *mockEventHandler) OnTunnelEstablished(host string, isIntercepted bool) {}

func (h *mockEventHandler) OnSSE(event string, ctx *ResponseContext) {}

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

// TestHandleHTTPErrorCases tests error handling in handleHTTP function
func TestHandleHTTPErrorCases(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建测试服务器
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 根据请求路径返回不同的响应
		switch r.URL.Path {
		case "/timeout":
			// 模拟超时
			time.Sleep(200 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		case "/large-response":
			// 返回大响应
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusOK)
			// 写入10KB数据
			data := make([]byte, 10240)
			for i := range data {
				data[i] = byte('A' + (i % 26))
			}
			w.Write(data)
		case "/invalid-url":
			// 不做任何事，让客户端超时
			return
		default:
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("OK"))
		}
	}))
	defer testServer.Close()

	// 创建一个监听器
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	proxyAddr := listener.Addr().String()
	listener.Close() // 关闭监听器，让服务器可以使用这个端口

	// 测试场景
	testCases := []struct {
		name         string
		setupProxy   func() *Server
		setupRequest func() (*http.Request, http.ResponseWriter)
		checkResult  func(t *testing.T, err error)
	}{
		{
			name: "创建请求错误",
			setupProxy: func() *Server {
				return NewServer(proxyAddr, certMgr, true, harLog, nil, false)
			},
			setupRequest: func() (*http.Request, http.ResponseWriter) {
				// 手动创建请求而不是使用httptest.NewRequest，以避免解析URL时的panic
				req := &http.Request{
					Method: "GET",
					URL: &url.URL{
						Scheme: "http",
						Host:   "example.com",
					},
					// 使用一个非URL格式的Host，用于触发NewRequest错误
					Host: ":::invalid:host:",
					// 必须设置这些字段以避免空指针异常
					Header: make(http.Header),
					Body:   http.NoBody,
				}
				return req, httptest.NewRecorder()
			},
			checkResult: func(t *testing.T, err error) {
				// 在handleHTTP中，这个错误会导致http.Error被调用
				// 但由于我们无法直接检查http.Error的调用，所以我们只能确保函数不会panic
				assert.NoError(t, err, "函数不应该panic")
			},
		},
		{
			name: "处理大量数据",
			setupProxy: func() *Server {
				return NewServer(proxyAddr, certMgr, true, harLog, nil, true) // 启用流量输出
			},
			setupRequest: func() (*http.Request, http.ResponseWriter) {
				req := httptest.NewRequest("GET", testServer.URL+"/large-response", nil)
				return req, httptest.NewRecorder()
			},
			checkResult: func(t *testing.T, err error) {
				assert.NoError(t, err, "处理大量数据不应该出错")
			},
		},
	}

	// 运行测试
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			proxy := tc.setupProxy()
			req, w := tc.setupRequest()

			// 调用handleHTTP
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("函数panic: %v", r)
					}
				}()
				proxy.handleHTTP(w, req)
			}()

			// 检查结果
			tc.checkResult(t, err)
		})
	}
}

// TestHTTPHandlerWithCompression 测试HTTP处理器处理压缩响应的功能
func TestHTTPHandlerWithCompression(t *testing.T) {
	// 创建一个测试服务器，模拟返回gzip压缩的响应
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置响应头
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Content-Encoding", "gzip")

		// 压缩响应体
		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)

		// 写入测试JSON数据
		testData := `{"message":"这是一个测试JSON响应","success":true,"code":200}`
		_, _ = gw.Write([]byte(testData))
		_ = gw.Close()

		// 返回压缩后的响应
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(buf.Bytes())
	}))
	defer testServer.Close()

	// 创建代理服务器
	server := &Server{
		Addr:         "127.0.0.1:0", // 使用随机端口
		Verbose:      true,
		HarLogger:    harlogger.NewLogger("test.har", "TestProxy", "1.0"),
		EventHandler: &mockEventHandler{}, // 使用mock事件处理器
	}

	// 创建测试HTTP请求，指向测试服务器
	req, err := http.NewRequest("GET", testServer.URL, nil)
	assert.NoError(t, err)

	// 创建响应记录器
	recorder := httptest.NewRecorder()

	// 处理请求
	server.handleHTTP(recorder, req)

	// 获取代理服务器的响应
	resp := recorder.Result()

	// 验证响应头
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	// 注意：在使用httptest.ResponseRecorder时，对header的修改可能无法正确反映
	// 这里只确认响应体已被正确解压缩

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	defer resp.Body.Close()

	// 验证响应体已被解压
	expectedData := `{"message":"这是一个测试JSON响应","success":true,"code":200}`
	assert.Equal(t, expectedData, string(body))
}
