package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
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
)

func TestServerHTTPHandlers(t *testing.T) {
	// 创建测试用的后端服务器
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求方法
		assert.Equal(t, "GET", r.Method)
		// 验证请求头
		assert.Equal(t, "example.com", r.Host)

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
		false,
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
		},
	}

	// 创建测试服务器
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Host", "example.com")
		w.WriteHeader(200)
		w.Write([]byte("Hello from HTTP server"))
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
	assert.Contains(t, string(body), "Hello from HTTP server")
}

func TestServerHTTPSHandlers(t *testing.T) {
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

func TestServerWithSSE(t *testing.T) {
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
		true, // 启用详细日志
		harLog,
		false,
		nil,
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

	// 创建模拟SSE服务器
	sseServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 设置SSE响应头
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(200)

		// 发送SSE事件
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("Expected ResponseWriter to be a Flusher")
		}

		// 发送几个事件
		for i := 0; i < 3; i++ {
			fmt.Fprintf(w, "event: message\n")
			fmt.Fprintf(w, "data: {\"message\": \"Event %d\"}\n\n", i)
			flusher.Flush()
			time.Sleep(10 * time.Millisecond)
		}
	}))
	defer sseServer.Close()

	// 发送SSE请求
	req, err := http.NewRequest("GET", sseServer.URL+"/events", nil)
	assert.NoError(t, err)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// 读取SSE响应
	scanner := bufio.NewScanner(resp.Body)
	eventCount := 0
	for scanner.Scan() && eventCount < 6 { // 每个事件有2行，共3个事件
		line := scanner.Text()
		if strings.HasPrefix(line, "data:") {
			eventCount++
		}
	}
	resp.Body.Close()

	// 确保我们收到了所有事件
	assert.True(t, eventCount > 0, "应该接收到至少一个SSE事件")
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

	// 创建代理服务器，启用流量输出
	server := NewServer(
		proxyAddr,
		certMgr,
		false,
		harLog,
		false,
		nil,
		true, // 启用流量输出
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

	// 创建测试服务器
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("Hello from test server with dump traffic"))
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
	assert.Contains(t, string(body), "Hello from test server with dump traffic")
}

func TestServerWithUpstreamProxy(t *testing.T) {
	// 创建上游代理服务器
	upstreamProxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodConnect {
			// 处理CONNECT请求
			w.WriteHeader(200)
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("不支持Hijacker")
			}
			conn, _, err := hijacker.Hijack()
			if err != nil {
				t.Fatalf("Hijack失败: %v", err)
			}
			defer conn.Close()

			// 连接到目标服务器
			targetConn, err := net.Dial("tcp", r.Host)
			if err != nil {
				t.Fatalf("连接目标服务器失败: %v", err)
			}
			defer targetConn.Close()

			// 双向转发数据
			go io.Copy(targetConn, conn)
			io.Copy(conn, targetConn)
		} else {
			// 处理普通HTTP请求
			targetURL := "http://" + r.Host + r.URL.Path
			if r.URL.RawQuery != "" {
				targetURL += "?" + r.URL.RawQuery
			}

			// 创建到目标服务器的请求
			targetReq, err := http.NewRequest(r.Method, targetURL, r.Body)
			if err != nil {
				http.Error(w, "创建目标请求失败", http.StatusInternalServerError)
				return
			}

			// 复制请求头
			for k, vv := range r.Header {
				for _, v := range vv {
					targetReq.Header.Add(k, v)
				}
			}

			// 发送请求到目标服务器
			client := &http.Client{}
			targetResp, err := client.Do(targetReq)
			if err != nil {
				http.Error(w, "请求目标服务器失败", http.StatusBadGateway)
				return
			}
			defer targetResp.Body.Close()

			// 复制响应头
			for k, vv := range targetResp.Header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}

			// 设置状态码
			w.WriteHeader(targetResp.StatusCode)

			// 复制响应体
			io.Copy(w, targetResp.Body)
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

	// 解析上游代理URL
	upstreamURL, _ := url.Parse(upstreamProxy.URL)

	// 创建代理服务器，使用上游代理
	server := NewServer(
		proxyAddr,
		certMgr,
		false,
		harLog,
		false,
		upstreamURL, // 使用上游代理
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

	// 创建测试服务器
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(200)
		w.Write([]byte("Hello from test server via upstream proxy"))
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
	assert.Contains(t, string(body), "Hello from test server via upstream proxy")
}

func TestUtilityFunctions(t *testing.T) {
	// 测试 isServerSentEvent 函数
	resp := &http.Response{
		Header: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		Request: &http.Request{
			URL: &url.URL{
				Path: "/events",
			},
		},
	}
	assert.True(t, isServerSentEvent(resp), "应该识别为SSE响应")

	// 测试非SSE响应
	resp.Header.Set("Content-Type", "text/plain")
	assert.False(t, isServerSentEvent(resp), "不应该识别为SSE响应")

	// 测试 JSON 流响应
	resp.Header.Set("Content-Type", "application/json")
	resp.Request.URL.Path = "/stream/5"
	assert.True(t, isServerSentEvent(resp), "JSON流应该识别为SSE响应")

	// 测试 OpenAI 流式 API
	resp.Header.Set("Content-Type", "application/json")
	resp.Request.URL.Path = "/v1/chat/completions"
	resp.Request.URL.RawQuery = "stream=true"
	assert.True(t, isServerSentEvent(resp), "OpenAI流式 API应该识别为SSE响应")

	// 测试 isSSERequest 函数
	req := &http.Request{
		Header: http.Header{
			"Accept": []string{"text/event-stream"},
		},
		URL: &url.URL{
			Path: "/events",
		},
	}
	assert.True(t, isSSERequest(req), "应该识别为SSE请求")

	// 测试非SSE请求
	req.Header.Set("Accept", "text/plain")
	req.URL.Path = "/api"
	assert.False(t, isSSERequest(req), "不应该识别为SSE请求")

	// 测试基于路径的SSE请求
	req.URL.Path = "/stream"
	assert.True(t, isSSERequest(req), "基于路径的SSE请求应该被识别")

	// 测试 isBinaryContent 函数
	assert.True(t, isBinaryContent([]byte{0x00, 0x01, 0x02, 0x03}, "application/octet-stream"), "应该识别为二进制内容")
	assert.False(t, isBinaryContent([]byte("Hello, world!"), "text/plain"), "不应该识别为二进制内容")

	// 测试基于内容类型的二进制检测
	assert.True(t, isBinaryContent([]byte("Hello"), "image/jpeg"), "图片应该被识别为二进制")
	assert.True(t, isBinaryContent([]byte("Hello"), "application/pdf"), "PDF应该被识别为二进制")

	// 测试空内容 - 空内容不应该被识别为二进制
	// 注意：在实际代码中，isBinaryContent 函数对空内容返回 false
	// 但在测试中我们需要确保这一点
	assert.False(t, isBinaryContent([]byte{}, "text/plain"), "空内容不应该被识别为二进制")

	// 测试 min 函数
	assert.Equal(t, 5, min(5, 10), "min(5, 10) 应该返回 5")
	assert.Equal(t, 5, min(10, 5), "min(10, 5) 应该返回 5")

	// 测试 readAndRestoreBody 函数
	testBody := io.NopCloser(strings.NewReader("test body content"))
	var bodySlot io.ReadCloser = testBody
	bodyBytes, err := readAndRestoreBody(&bodySlot, -1)
	assert.NoError(t, err)
	assert.Equal(t, "test body content", string(bodyBytes))

	// 测试空体
	var emptyBody io.ReadCloser = nil
	bodyBytes, err = readAndRestoreBody(&emptyBody, -1)
	assert.NoError(t, err)
	assert.Nil(t, bodyBytes)
}

func TestResponseBodyTee(t *testing.T) {
	// 创建一个模拟的响应写入器
	buf := &bytes.Buffer{}

	// 创建一个模拟的Flusher
	flusher := &mockFlusher{}

	// 创建 ResponseBodyTee
	tee := &ResponseBodyTee{
		buffer:  &bytes.Buffer{},
		writer:  buf,
		flusher: flusher,
	}

	// 测试写入数据
	n, err := tee.Write([]byte("test data"))
	assert.NoError(t, err)
	assert.Equal(t, 9, n)

	// 检查数据是否写入了原始写入器
	assert.Equal(t, "test data", buf.String())

	// 检查数据是否写入了缓冲区
	assert.Equal(t, "test data", tee.GetBuffer().String())

	// 检查是否调用了Flush
	assert.True(t, flusher.flushed)
}

// mockFlusher 是一个模拟的http.Flusher实现
type mockFlusher struct {
	flushed bool
}

func (f *mockFlusher) Flush() {
	f.flushed = true
}

func TestHeaderInterceptingTransport(t *testing.T) {
	// 创建一个模拟的RoundTripper
	mockTransport := &mockRoundTripper{
		response: &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("test response")),
		},
	}

	// 创建一个回调函数
	callbackCalled := false
	callback := func(resp *http.Response) (*http.Response, error) {
		callbackCalled = true
		// 修改响应
		resp.Header.Set("X-Modified", "true")
		return resp, nil
	}

	// 创建 headerInterceptingTransport
	transport := &headerInterceptingTransport{
		base:     mockTransport,
		verbose:  true,
		callback: callback,
	}

	// 创建请求
	req, _ := http.NewRequest("GET", "http://example.com", nil)

	// 执行 RoundTrip
	resp, err := transport.RoundTrip(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// 检查回调是否被调用
	assert.True(t, callbackCalled)

	// 检查响应是否被修改
	assert.Equal(t, "true", resp.Header.Get("X-Modified"))

	// 测试没有回调的情况
	transport.callback = nil
	resp, err = transport.RoundTrip(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
}

// mockRoundTripper 是一个模拟的http.RoundTripper实现
type mockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestEarlySSEDetector(t *testing.T) {
	// 创建一个模拟的响应
	mockResp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("data: test\n\n")),
		Request:    &http.Request{URL: &url.URL{Path: "/events"}},
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	mockResp.Header.Set("Content-Type", "text/event-stream")

	// 创建一个模拟的RoundTripper
	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	// 创建一个模拟的Server
	server := &Server{
		Verbose: true,
	}

	// 创建 earlySSEDetector
	detector := &earlySSEDetector{
		base:           mockTransport,
		responseWriter: &mockResponseWriter{},
		server:         server,
		verbose:        true,
	}

	// 创建请求
	req, _ := http.NewRequest("GET", "http://example.com/events", nil)
	req.Header.Set("Accept", "text/event-stream")

	// 执行 RoundTrip
	resp, err := detector.RoundTrip(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// 检查响应头
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
	assert.Equal(t, "chunked", resp.Header.Get("Transfer-Encoding"))

	// 测试非SSE响应
	mockResp.Header.Set("Content-Type", "text/plain")
	resp, err = detector.RoundTrip(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, "text/plain", resp.Header.Get("Content-Type"))
}

// mockResponseWriter 是一个模拟的http.ResponseWriter实现
type mockResponseWriter struct {
	headers http.Header
	status  int
	buffer  bytes.Buffer
}

func (m *mockResponseWriter) Header() http.Header {
	if m.headers == nil {
		m.headers = make(http.Header)
	}
	return m.headers
}

func (m *mockResponseWriter) Write(b []byte) (int, error) {
	return m.buffer.Write(b)
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.status = statusCode
}

// 测试 logToHAR 函数
func TestLogToHAR(t *testing.T) {
	// 创建一个模拟的Server
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")
	server := &Server{
		HarLogger: harLog,
		Verbose:   true,
	}

	// 创建请求和响应
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("test response")),
		Request:    req,
	}

	// 测试正常记录
	startTime := time.Now()
	timeTaken := 100 * time.Millisecond
	server.logToHAR(req, resp, startTime, timeTaken, false)

	// 测试SSE响应记录
	server.logToHAR(req, resp, startTime, timeTaken, true)

	// 测试响应为nil的情况
	server.logToHAR(req, nil, startTime, timeTaken, false)
}

func TestDumpRequestAndResponseBody(t *testing.T) {
	// 创建一个模拟的Server
	server := &Server{
		DumpTraffic: true,
	}

	// 测试请求体输出
	req, _ := http.NewRequest("POST", "http://example.com", strings.NewReader("test request body"))
	req.Header.Set("Content-Type", "text/plain")
	server.dumpRequestBody(req)

	// 测试二进制请求体
	req, _ = http.NewRequest("POST", "http://example.com", strings.NewReader("\x00\x01\x02\x03"))
	req.Header.Set("Content-Type", "application/octet-stream")
	server.dumpRequestBody(req)

	// 测试响应体输出
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("test response body")),
		Proto:      "HTTP/1.1",
		Status:     "200 OK",
	}
	resp.Header.Set("Content-Type", "text/plain")
	server.dumpResponseBody(resp)

	// 测试二进制响应体
	resp = &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("\x00\x01\x02\x03")),
		Proto:      "HTTP/1.1",
		Status:     "200 OK",
	}
	resp.Header.Set("Content-Type", "application/octet-stream")
	server.dumpResponseBody(resp)

	// 测试禁用流量输出
	server.DumpTraffic = false
	server.dumpRequestBody(req)
	server.dumpResponseBody(resp)

	// 测试响应为nil
	server.DumpTraffic = true
	server.dumpResponseBody(nil)
}

func TestHandleHTTP2(t *testing.T) {
	// 创建一个模拟的Server
	server := &Server{
		Verbose: true,
	}

	// 创建一个传输器
	transport := &http.Transport{}

	// 配置HTTP/2支持
	server.handleHTTP2(transport)

	// 测试非详细模式
	server.Verbose = false
	server.handleHTTP2(transport)
}

func TestLogSSEEvent(t *testing.T) {
	// 测试不同类型的SSE事件
	logSSEEvent("data: test data", true)
	logSSEEvent("event: message", true)
	logSSEEvent("id: 123", true)
	logSSEEvent("retry: 1000", true)
	logSSEEvent("custom: value", true)
	logSSEEvent("", true) // 空行

	// 测试非详细模式
	logSSEEvent("data: test data", false)
}

// 测试 handleSSE 函数
func TestHandleSSE(t *testing.T) {
	// 创建一个模拟的Server
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")
	server := &Server{
		HarLogger: harLog,
		Verbose:   true,
	}

	// 创建一个模拟的响应
	resp := &http.Response{
		StatusCode: 200,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("data: test\n\nevent: message\ndata: {\"message\": \"test\"}\n\n")),
		Request:    &http.Request{URL: &url.URL{Path: "/events"}},
		ProtoMajor: 1,
		ProtoMinor: 1,
	}
	resp.Header.Set("Content-Type", "text/event-stream")

	// 创建一个模拟的ResponseWriter
	w := &mockResponseWriterFlusher{}

	// 测试 handleSSE 函数
	err := server.handleSSE(w, resp)
	assert.NoError(t, err)

	// 检查响应头
	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	assert.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	assert.Equal(t, "keep-alive", w.Header().Get("Connection"))

	// 测试启用流量输出
	server.DumpTraffic = true
	resp.Body = io.NopCloser(strings.NewReader("data: test\n\nevent: message\ndata: {\"message\": \"test\"}\n\n"))
	err = server.handleSSE(w, resp)
	assert.NoError(t, err)
}

// mockResponseWriterFlusher 是一个模拟的http.ResponseWriter和http.Flusher实现
type mockResponseWriterFlusher struct {
	mockResponseWriter
}

func (m *mockResponseWriterFlusher) Flush() {}

// 测试 mayBeServerSentEvent 函数
func TestMayBeServerSentEvent(t *testing.T) {
	// 创建一个请求
	req, _ := http.NewRequest("GET", "http://example.com/events", nil)
	req.Header.Set("Accept", "text/event-stream")

	// 测试可能是SSE请求
	assert.True(t, mayBeServerSentEvent(req))

	// 测试非SSE请求
	req.Header.Set("Accept", "text/plain")
	assert.False(t, mayBeServerSentEvent(req))
}

// 测试 logHeader 函数
func TestLogHeader(t *testing.T) {
	// 创建一个测试头部
	header := make(http.Header)
	header.Add("Content-Type", "text/plain")
	header.Add("X-Test", "test-value")

	// 测试输出头部
	logHeader(header, "Test Headers:")
}

// 测试 NewServer 函数
func TestNewServer(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建代理服务器
	server := NewServer(
		":8080",
		certMgr,
		true,
		harLog,
		true,
		nil,
		true,
	)

	// 检查服务器属性
	assert.Equal(t, ":8080", server.Addr)
	assert.Equal(t, certMgr, server.CertManager)
	assert.True(t, server.Verbose)
	assert.Equal(t, harLog, server.HarLogger)
	assert.True(t, server.EnableMITM)
	assert.Nil(t, server.UpstreamProxy)
	assert.True(t, server.DumpTraffic)
}
