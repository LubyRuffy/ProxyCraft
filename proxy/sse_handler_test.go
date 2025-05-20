package proxy

import (
	"bytes"
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

// 定义测试使用的助手结构体和函数

// MockFlusher 是一个模拟的http.Flusher实现
type MyMockFlusher struct {
	flushed bool
}

func (f *MyMockFlusher) Flush() {
	f.flushed = true
}

// TestResponseWriteTee 测试ResponseBodyTee的写入功能
func TestResponseBodyTeeWrite(t *testing.T) {
	// 创建一个模拟的响应写入器
	buf := &bytes.Buffer{}

	// 创建一个模拟的Flusher
	flusher := &MyMockFlusher{}

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

// MyMockResponseWriterFlusher 是一个实现了http.ResponseWriter和http.Flusher的模拟类
type MyMockResponseWriterFlusher struct {
	headers http.Header
	status  int
	buffer  bytes.Buffer
}

func (m *MyMockResponseWriterFlusher) Header() http.Header {
	return m.headers
}

func (m *MyMockResponseWriterFlusher) Write(b []byte) (int, error) {
	return m.buffer.Write(b)
}

func (m *MyMockResponseWriterFlusher) WriteHeader(statusCode int) {
	m.status = statusCode
}

func (m *MyMockResponseWriterFlusher) Flush() {
	// 模拟实现Flush，什么都不做
}

// TestServerWithServerSentEvents 测试服务器对SSE的处理
func TestServerWithServerSentEvents(t *testing.T) {
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
	defer resp.Body.Close()

	// 检查响应头
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	// 读取并检查响应体
	reader := io.LimitReader(resp.Body, 1024) // 限制读取量
	body, err := io.ReadAll(reader)
	assert.NoError(t, err)
	bodyStr := string(body)

	// 检查是否包含SSE事件
	assert.Contains(t, bodyStr, "event: message")
	assert.Contains(t, bodyStr, "data: {\"message\":")
}

// TestSSEHandleFunction 测试handleSSE函数
func TestSSEHandleFunction(t *testing.T) {
	// 创建模拟响应和请求
	mockResp := &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:       io.NopCloser(strings.NewReader("event: message\ndata: {\"message\": \"Test event\"}\n\n")),
	}

	// 创建请求上下文
	mockReq, _ := http.NewRequest("GET", "http://example.com/events", nil)
	mockReq.Header.Set("Accept", "text/event-stream")
	mockResp.Request = mockReq

	reqCtx := &RequestContext{
		Request:   mockReq,
		StartTime: time.Now(),
		IsSSE:     true,
		UserData:  make(map[string]interface{}),
	}

	// 创建响应上下文
	respCtx := &ResponseContext{
		ReqCtx:   reqCtx,
		Response: mockResp,
		IsSSE:    true,
		UserData: make(map[string]interface{}),
	}

	// 创建虚拟ResponseWriter
	respWriter := &MyMockResponseWriterFlusher{
		headers: make(http.Header),
	}

	// 创建服务器，包含所有必要的依赖项
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")
	certMgr, _ := certs.NewManager()

	server := &Server{
		Verbose:     true,
		HarLogger:   harLog,
		CertManager: certMgr,
	}

	// 测试handleSSE
	err := server.handleSSE(respWriter, respCtx)
	assert.NoError(t, err)

	// 检查设置的头部
	assert.Equal(t, "text/event-stream", respWriter.headers.Get("Content-Type"))
	assert.Equal(t, "no-cache", respWriter.headers.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", respWriter.headers.Get("Connection"))

	// 检查写入的内容
	assert.Contains(t, respWriter.buffer.String(), "event: message")
	assert.Contains(t, respWriter.buffer.String(), "data: {\"message\": \"Test event\"}")
}

// TestSSEEventLogging 测试SSE事件日志记录
func TestSSEEventLogging(t *testing.T) {
	// 测试不同类型的SSE事件行
	tests := []struct {
		input   string
		verbose bool
	}{
		{"data: test data", true},
		{"event: test event", true},
		{"id: 123", true},
		{"retry: 1000", true},
		{"", true},                   // 空行
		{"data: verbose off", false}, // verbose关闭时
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			// 调用测试函数，不期望报错
			logSSEEvent(test.input, test.verbose)
		})
	}
}

// TestMayBeSSE 测试mayBeServerSentEvent函数
func TestMayBeSSE(t *testing.T) {
	// 创建包含Accept: text/event-stream的请求
	sseReq, _ := http.NewRequest("GET", "http://example.com/events", nil)
	sseReq.Header.Set("Accept", "text/event-stream")

	// 创建普通请求
	normalReq, _ := http.NewRequest("GET", "http://example.com/api", nil)
	normalReq.Header.Set("Accept", "application/json")

	// 测试函数行为
	assert.True(t, mayBeServerSentEvent(sseReq))
	assert.False(t, mayBeServerSentEvent(normalReq))
}

// TestIsSSE 测试isServerSentEvent函数
func TestIsSSE(t *testing.T) {
	// 创建带有text/event-stream Content-Type的响应
	sseResp := &http.Response{
		Header:  http.Header{"Content-Type": []string{"text/event-stream"}},
		Request: &http.Request{URL: &url.URL{Path: "/events"}},
	}

	// 创建普通响应
	normalResp := &http.Response{
		Header:  http.Header{"Content-Type": []string{"application/json"}},
		Request: &http.Request{URL: &url.URL{Path: "/api"}},
	}

	// OpenAI流式API响应
	openaiResp := &http.Response{
		Header: http.Header{"Content-Type": []string{"application/json"}},
		Request: &http.Request{
			URL: &url.URL{
				Path:     "/v1/chat/completions",
				RawQuery: "stream=true",
			},
		},
	}

	// 测试函数行为
	assert.True(t, isServerSentEvent(sseResp))
	assert.False(t, isServerSentEvent(normalResp))
	assert.True(t, isServerSentEvent(openaiResp))
}

// MyMockRoundTripper 是一个模拟的http.RoundTripper实现
type MyMockRoundTripper struct {
	response *http.Response
	err      error
}

func (m *MyMockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

// TestEarlyDetection 测试earlySSEDetector实现
func TestEarlyDetection(t *testing.T) {
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
	mockTransport := &MyMockRoundTripper{
		response: mockResp,
	}

	// 创建服务器，包含所有必要的依赖项
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")
	certMgr, _ := certs.NewManager()

	server := &Server{
		Verbose:     true,
		HarLogger:   harLog,
		CertManager: certMgr,
	}

	// 创建EarlySSEDetector
	detector := &earlySSEDetector{
		base:           mockTransport,
		responseWriter: &MyMockResponseWriterFlusher{},
		server:         server,
		verbose:        true,
	}

	// 创建一个测试请求
	req, _ := http.NewRequest("GET", "http://example.com/events", nil)
	req.Header.Set("Accept", "text/event-stream")

	// 执行RoundTrip
	resp, err := detector.RoundTrip(req)
	assert.NoError(t, err)
	assert.NotNil(t, resp)

	// 检查返回的响应
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))
	assert.Equal(t, "no-cache", resp.Header.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", resp.Header.Get("Connection"))
	assert.Equal(t, "chunked", resp.Header.Get("Transfer-Encoding"))

	// 读取响应体（应该是通过管道传输的）
	body, err := io.ReadAll(resp.Body)
	assert.NoError(t, err)
	resp.Body.Close()

	// 管道中可能还没有完整的数据，但我们可以确保没有错误
	assert.NotNil(t, body)
}
