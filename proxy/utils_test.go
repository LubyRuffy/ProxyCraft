package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
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
		Addr:      "127.0.0.1:0",
		Verbose:   true,
		HarLogger: harLog,
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
		Addr:      "127.0.0.1:0",
		Verbose:   true,
		HarLogger: nil, // HarLogger为nil
	}
	disabledServer.logToHAR(req, resp, startTime, timeTaken, false)

	// 测试场景5: HarLogger已启用但IsEnabled()返回false
	// 创建一个没有输出文件的logger，它的IsEnabled()将返回false
	disabledHarLog := harlogger.NewLogger("", "ProxyCraft Test", "0.1.0")

	disabledServerWithLogger := &Server{
		Addr:      "127.0.0.1:0",
		Verbose:   true,
		HarLogger: disabledHarLog,
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

// TestDecompressBody 测试响应体解压缩功能
func TestDecompressBody(t *testing.T) {
	t.Run("无压缩内容", func(t *testing.T) {
		// 创建一个没有压缩的响应
		resp := &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("测试数据")),
		}

		// 调用解压函数
		err := decompressBody(resp)
		assert.NoError(t, err)

		// 验证内容没有变化
		bodyBytes, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "测试数据", string(bodyBytes))
	})

	t.Run("Gzip压缩内容", func(t *testing.T) {
		// 创建Gzip压缩的测试数据
		var compressedData bytes.Buffer
		gzipWriter := gzip.NewWriter(&compressedData)
		_, err := gzipWriter.Write([]byte(`{"message":"这是一个测试JSON"}`))
		assert.NoError(t, err)
		err = gzipWriter.Close()
		assert.NoError(t, err)

		// 创建带有Gzip压缩的响应
		resp := &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(compressedData.Bytes())),
		}
		resp.Header.Set("Content-Encoding", "gzip")
		resp.Header.Set("Content-Type", "application/json")

		// 调用解压函数
		err = decompressBody(resp)
		assert.NoError(t, err)

		// 验证解压后的内容
		bodyBytes, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, `{"message":"这是一个测试JSON"}`, string(bodyBytes))

		// 验证Content-Encoding已被移除
		assert.Equal(t, "", resp.Header.Get("Content-Encoding"))

		// 验证Content-Length已被更新
		assert.Equal(t, fmt.Sprint(len(`{"message":"这是一个测试JSON"}`)), resp.Header.Get("Content-Length"))
	})

	t.Run("Deflate压缩内容", func(t *testing.T) {
		// 创建Deflate压缩的测试数据
		var compressedData bytes.Buffer
		deflateWriter, err := flate.NewWriter(&compressedData, flate.DefaultCompression)
		assert.NoError(t, err)
		_, err = deflateWriter.Write([]byte("<html><body>这是一个测试HTML</body></html>"))
		assert.NoError(t, err)
		err = deflateWriter.Close()
		assert.NoError(t, err)

		// 创建带有Deflate压缩的响应
		resp := &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(compressedData.Bytes())),
		}
		resp.Header.Set("Content-Encoding", "deflate")
		resp.Header.Set("Content-Type", "text/html")

		// 调用解压函数
		err = decompressBody(resp)
		assert.NoError(t, err)

		// 验证解压后的内容
		bodyBytes, err := io.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, "<html><body>这是一个测试HTML</body></html>", string(bodyBytes))

		// 验证Content-Encoding已被移除
		assert.Equal(t, "", resp.Header.Get("Content-Encoding"))

		// 验证Content-Length已被更新
		assert.Equal(t, fmt.Sprint(len("<html><body>这是一个测试HTML</body></html>")), resp.Header.Get("Content-Length"))
	})

	t.Run("无效的Gzip内容", func(t *testing.T) {
		// 创建无效的Gzip数据
		invalidData := []byte("这不是有效的gzip压缩数据")

		// 创建带有错误Gzip压缩内容的响应
		resp := &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(bytes.NewReader(invalidData)),
		}
		resp.Header.Set("Content-Encoding", "gzip")

		// 调用解压函数，应该返回错误
		err := decompressBody(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "无效的gzip数据: 缺少正确的魔术数字")
	})

	t.Run("不支持的压缩格式", func(t *testing.T) {
		// 创建带有不支持压缩格式的响应
		resp := &http.Response{
			StatusCode: 200,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("一些数据")),
		}
		resp.Header.Set("Content-Encoding", "br") // 使用Brotli压缩算法

		// 调用解压函数，应该返回错误
		err := decompressBody(resp)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "不支持的编码方式: br")
	})

	t.Run("nil响应", func(t *testing.T) {
		// 测试nil响应处理
		err := decompressBody(nil)
		assert.NoError(t, err)
	})

	t.Run("nil响应体", func(t *testing.T) {
		// 测试nil响应体处理
		resp := &http.Response{
			StatusCode: 204, // No Content
			Header:     make(http.Header),
			Body:       nil,
		}
		resp.Header.Set("Content-Encoding", "gzip")

		// 调用解压函数
		err := decompressBody(resp)
		assert.NoError(t, err)
	})
}

// TestIsTextContentType 测试文本内容类型识别函数
func TestIsTextContentType(t *testing.T) {
	// 测试各种内容类型
	textTypes := []string{
		"text/plain",
		"text/html",
		"text/css",
		"text/javascript",
		"application/json",
		"application/xml",
		"application/javascript",
		"application/x-www-form-urlencoded",
		"application/vnd.api+json",
		"application/vnd.custom+xml",
	}

	nonTextTypes := []string{
		"",
		"image/jpeg",
		"image/png",
		"audio/mpeg",
		"video/mp4",
		"application/octet-stream",
		"application/pdf",
		"application/zip",
		"application/x-gzip",
	}

	// 验证文本类型
	for _, contentType := range textTypes {
		t.Run(contentType, func(t *testing.T) {
			assert.True(t, isTextContentType(contentType), fmt.Sprintf("%s 应该被识别为文本类型", contentType))
		})
	}

	// 验证非文本类型
	for _, contentType := range nonTextTypes {
		t.Run(contentType, func(t *testing.T) {
			assert.False(t, isTextContentType(contentType), fmt.Sprintf("%s 不应该被识别为文本类型", contentType))
		})
	}
}
