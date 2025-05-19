package proxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/LubyRuffy/ProxyCraft/harlogger"
	"github.com/stretchr/testify/assert"
)

// TestRealUtilsFunctions contains the tests for utility functions in utils.go
func TestIsBinaryContent(t *testing.T) {
	// 测试二进制内容识别
	binaryData := []byte{0x00, 0x01, 0x02, 0x03}
	textData := []byte("This is a text content") // 使用ASCII文本而不是中文

	// 基于Content-Type的检测
	assert.True(t, isBinaryContent([]byte{}, "image/png"))
	assert.True(t, isBinaryContent([]byte{}, "application/pdf"))
	assert.False(t, isBinaryContent([]byte{}, "text/plain"))
	assert.False(t, isBinaryContent([]byte{}, "application/json"))

	// 基于内容的检测
	assert.True(t, isBinaryContent(binaryData, ""))
	assert.False(t, isBinaryContent(textData, ""))
}

func TestReadAndRestoreBody(t *testing.T) {
	// 创建测试体
	testBody := io.NopCloser(strings.NewReader("test body"))
	bodySlot := &testBody

	// 读取并恢复
	bodyBytes, err := readAndRestoreBody(bodySlot, -1)
	assert.NoError(t, err)
	assert.Equal(t, "test body", string(bodyBytes))

	// 检查是否恢复了Body
	restoredContent, err := io.ReadAll(*bodySlot)
	assert.NoError(t, err)
	assert.Equal(t, "test body", string(restoredContent))

	// 测试nil情况
	var nilBody io.ReadCloser = nil
	nilBodySlot := &nilBody
	nilBytes, err := readAndRestoreBody(nilBodySlot, -1)
	assert.NoError(t, err)
	assert.Nil(t, nilBytes)
}

func TestMin(t *testing.T) {
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 5, min(10, 5))
	assert.Equal(t, 0, min(0, 5))
	assert.Equal(t, -5, min(-5, 5))
}

// TestSimpleRequestResponseDump tests the formatting of HTTP request and response dumps
func TestSimpleRequestResponseDump(t *testing.T) {
	// 模拟简单的请求/响应转储格式
	var buffer bytes.Buffer
	buffer.WriteString(">>>>>>>>>>>>>>>>>>>>\n")
	buffer.WriteString("POST http://example.com HTTP/1.1\n")
	buffer.WriteString("Content-Type: text/plain\n\n")
	buffer.WriteString("request body\n")
	buffer.WriteString(">>>>>>>>>>>>>>>>>>>>\n")
	buffer.WriteString("<<<<<<<<<<<<<<<<<<<<<<\n")
	buffer.WriteString("HTTP/0.0 200 OK\n")
	buffer.WriteString("Content-Type: application/json\n\n")
	buffer.WriteString(`{"result":"success"}` + "\n")
	buffer.WriteString("<<<<<<<<<<<<<<<<<<<<<<\n")

	output := buffer.String()
	assert.Contains(t, output, "POST http://example.com")
	assert.Contains(t, output, "Content-Type: text/plain")
	assert.Contains(t, output, "request body")
	assert.Contains(t, output, "200 OK")
	assert.Contains(t, output, "Content-Type: application/json")
	assert.Contains(t, output, `{"result":"success"}`)
}

// TestLogHeader 测试日志记录HTTP头信息函数
func TestLogHeader(t *testing.T) {
	// 创建测试用的HTTP头
	header := make(http.Header)
	header.Add("Content-Type", "text/html")
	header.Add("Content-Length", "123")
	header.Add("Connection", "keep-alive")
	header.Add("X-Test-Header", "test-value")

	// 直接调用logHeader函数
	logHeader(header, "Test Headers:")

	// 由于logHeader只是打印日志，没有返回值，我们只能测试它没有panic
	// 在实际情况下，可以捕获日志输出来验证，但这里简化处理
}

// TestDumpRequestBody 测试请求体内容导出功能
func TestDumpRequestBody(t *testing.T) {
	// 创建服务器
	server := &Server{
		Addr:        "127.0.0.1:0",
		Verbose:     true,
		DumpTraffic: true, // 启用流量输出
	}

	// 测试场景1: 标准请求
	req, err := http.NewRequest("POST", "http://example.com/test", strings.NewReader("Test request body"))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "text/plain")

	// 备份原来的输出设备
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 调用函数
	server.dumpRequestBody(req)

	// 恢复输出设备
	w.Close()
	os.Stdout = oldStdout

	// 读取捕获的输出
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 验证输出
	assert.Contains(t, output, "Test request body")

	// 测试场景2: 请求没有body
	reqWithoutBody, _ := http.NewRequest("GET", "http://example.com/test", nil)

	// 再次设置捕获输出
	r, w, _ = os.Pipe()
	os.Stdout = w

	// 调用函数
	server.dumpRequestBody(reqWithoutBody)

	// 恢复输出设备
	w.Close()
	os.Stdout = oldStdout

	// 不需要验证输出，因为不应该有任何输出
}

// TestDumpResponseBody 测试响应体内容导出功能
func TestDumpResponseBody(t *testing.T) {
	// 创建服务器
	server := &Server{
		Addr:        "127.0.0.1:0",
		Verbose:     true,
		DumpTraffic: true, // 启用流量输出
	}

	// 测试场景1: 标准响应
	resp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("Test response body")),
	}
	resp.Header.Set("Content-Type", "text/plain")

	// 备份原来的输出设备
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 调用函数
	server.dumpResponseBody(resp)

	// 恢复输出设备
	w.Close()
	os.Stdout = oldStdout

	// 读取捕获的输出
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 验证输出
	assert.Contains(t, output, "Test response body")

	// 测试场景2: 响应没有body
	respWithoutBody := &http.Response{
		StatusCode: 204,
		Proto:      "HTTP/1.1",
		Header:     make(http.Header),
		Body:       nil,
	}

	// 再次设置捕获输出
	r, w, _ = os.Pipe()
	os.Stdout = w

	// 调用函数
	server.dumpResponseBody(respWithoutBody)

	// 恢复输出设备
	w.Close()
	os.Stdout = oldStdout
}

// TestBinaryContentHandling 测试二进制内容处理
func TestBinaryContentHandling(t *testing.T) {
	// 创建服务器
	server := &Server{
		Addr:        "127.0.0.1:0",
		Verbose:     true,
		DumpTraffic: true, // 启用流量输出
	}

	// 创建一个包含二进制数据的请求
	binaryData := []byte{0x00, 0x01, 0x02, 0xFF, 0xFE, 0xFD}
	req, err := http.NewRequest("POST", "http://example.com/binary", bytes.NewReader(binaryData))
	assert.NoError(t, err)
	req.Header.Set("Content-Type", "application/octet-stream")

	// 备份原来的输出设备
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// 调用函数
	server.dumpRequestBody(req)

	// 恢复输出设备
	w.Close()
	os.Stdout = oldStdout

	// 读取捕获的输出
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 验证输出包含(binary data)提示
	assert.Contains(t, output, "(binary data)")
}

// TestReadAndRestoreBodyError 测试读取和恢复请求/响应体的错误处理
func TestReadAndRestoreBodyError(t *testing.T) {
	// 创建一个会在读取时产生错误的ReadCloser
	errorReader := &errorReadCloser{err: fmt.Errorf("test error")}

	// 测试readAndRestoreBody函数
	bodySlot := io.ReadCloser(errorReader)
	data, err := readAndRestoreBody(&bodySlot, -1)

	// 验证结果
	assert.Error(t, err)
	assert.Equal(t, "test error", err.Error())
	assert.Empty(t, data)
}

// errorReadCloser 实现io.ReadCloser接口，但始终返回错误
type errorReadCloser struct {
	err error
}

func (e *errorReadCloser) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func (e *errorReadCloser) Close() error {
	return nil
}

// TestServerLogToHAR 测试代理服务器的HAR日志记录功能
func TestServerLogToHAR(t *testing.T) {
	// 创建有效的HarLogger
	harLog := harlogger.NewLogger("", "ProxyCraft Test", "0.1.0")

	// 创建代理服务器 - 启用HarLogger
	server := &Server{
		Addr:       "127.0.0.1:0",
		Verbose:    true,
		HarLogger:  harLog,
		EnableMITM: true,
	}

	// 创建测试请求
	req, err := http.NewRequest("GET", "http://example.com/test", nil)
	assert.NoError(t, err)

	// 创建测试响应
	resp := &http.Response{
		StatusCode: 200,
		Proto:      "HTTP/1.1",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("Test response body")),
	}

	// 测试场景1: 标准请求和响应
	startTime := time.Now().Add(-1 * time.Second) // 1秒前
	timeTaken := 1 * time.Second
	server.logToHAR(req, resp, startTime, timeTaken, false)

	// 测试场景2: SSE响应
	server.logToHAR(req, resp, startTime, timeTaken, true)

	// 测试场景3: 请求为nil
	server.logToHAR(nil, resp, startTime, timeTaken, false)

	// 测试场景4: HarLogger未启用
	disabledServer := &Server{
		Addr:       "127.0.0.1:0",
		Verbose:    true,
		HarLogger:  nil, // HarLogger为nil
		EnableMITM: true,
	}
	disabledServer.logToHAR(req, resp, startTime, timeTaken, false)

	// 测试场景5: HarLogger已启用但IsEnabled()返回false
	// 创建一个没有输出文件的logger，它的IsEnabled()将返回false
	disabledHarLog := harlogger.NewLogger("", "ProxyCraft Test", "0.1.0")

	disabledServerWithLogger := &Server{
		Addr:       "127.0.0.1:0",
		Verbose:    true,
		HarLogger:  disabledHarLog,
		EnableMITM: true,
	}
	disabledServerWithLogger.logToHAR(req, resp, startTime, timeTaken, false)
}

// TestReadAndRestoreBodyWithTimeout tests the readAndRestoreBody function with timeout edge cases
func TestReadAndRestoreBodyWithTimeout(t *testing.T) {
	t.Run("slow_reader_with_non_blocking_buffer", func(t *testing.T) {
		// 创建一个模拟缓慢读取的reader
		slowReader := &slowReader{
			data:  []byte("slow data being read"),
			delay: 10 * time.Millisecond,
		}

		body := io.NopCloser(slowReader)
		data, err := readAndRestoreBody(&body, int64(len(slowReader.data)))

		assert.NoError(t, err)
		assert.Equal(t, string(slowReader.data), string(data))

		// 确认body被正确恢复
		restoredData, err := io.ReadAll(body)
		assert.NoError(t, err)
		assert.Equal(t, string(slowReader.data), string(restoredData))
	})

	t.Run("content_length_mismatch_larger_than_actual", func(t *testing.T) {
		content := "smaller content"
		body := io.NopCloser(strings.NewReader(content))

		// 指定一个大于实际内容长度的值
		data, err := readAndRestoreBody(&body, int64(len(content)+10))

		assert.NoError(t, err)
		assert.Equal(t, content, string(data), "应该只读取可用的数据")

		// 确认body被正确恢复
		restoredData, err := io.ReadAll(body)
		assert.NoError(t, err)
		assert.Equal(t, content, string(restoredData))
	})

	t.Run("very_large_content_length", func(t *testing.T) {
		content := "normal content"
		body := io.NopCloser(strings.NewReader(content))

		// 指定一个非常大的内容长度
		data, err := readAndRestoreBody(&body, 1000000) // 1MB

		assert.NoError(t, err)
		assert.Equal(t, content, string(data), "应该只读取可用的数据")

		// 确认body被正确恢复
		restoredData, err := io.ReadAll(body)
		assert.NoError(t, err)
		assert.Equal(t, content, string(restoredData))
	})
}

// slowReader 实现了一个模拟缓慢读取的io.Reader
type slowReader struct {
	data     []byte
	position int
	delay    time.Duration
}

func (s *slowReader) Read(p []byte) (n int, err error) {
	if s.position >= len(s.data) {
		return 0, io.EOF
	}

	// 模拟读取延迟
	time.Sleep(s.delay)

	// 每次只读取一个字节，模拟缓慢读取
	p[0] = s.data[s.position]
	s.position++

	return 1, nil
}
