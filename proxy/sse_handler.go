package proxy

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// ResponseBodyTee 是一个结构，用于同时将数据写入两个目的地
type ResponseBodyTee struct {
	buffer  *bytes.Buffer // 用于收集完整的响应体
	writer  io.Writer     // 原始的响应写入器
	flusher http.Flusher  // 用于刷新数据
}

// Write 实现 io.Writer 接口
func (t *ResponseBodyTee) Write(p []byte) (n int, err error) {
	// 写入原始响应写入器
	n, err = t.writer.Write(p)
	if err != nil {
		return n, err
	}

	// 同时写入缓冲区
	_, bufErr := t.buffer.Write(p)
	if bufErr != nil {
		// 如果缓冲区写入失败，仅记录日志，不影响原始写入
		log.Printf("[SSE] Error writing to buffer: %v", bufErr)
	}

	// 刷新数据
	if t.flusher != nil {
		t.flusher.Flush()
	}

	return n, nil
}

// GetBuffer 返回收集到的完整数据
func (t *ResponseBodyTee) GetBuffer() *bytes.Buffer {
	return t.buffer
}

// handleSSE handles Server-Sent Events responses
func (s *Server) handleSSE(w http.ResponseWriter, respCtx *ResponseContext) error {
	// 记录开始时间，用于后续的 HAR 记录
	startTime := time.Now()

	// Set appropriate headers for SSE
	for k, vv := range respCtx.Response.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// Ensure critical headers are set for SSE streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Del("Content-Length") // Remove Content-Length to ensure chunked encoding

	// 添加协议版本头以便前端识别
	w.Header().Add("X-Protocol", respCtx.Response.Proto)

	// Set the status code
	w.WriteHeader(respCtx.Response.StatusCode)

	// Create a flusher if the ResponseWriter supports it
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Flush headers immediately
	flusher.Flush()

	// Log SSE handling
	if s.Verbose {
		log.Printf("[SSE] Handling Server-Sent Events stream")
	}

	// 创建一个 ResponseBodyTee 来同时处理流和记录数据
	tee := &ResponseBodyTee{
		buffer:  &bytes.Buffer{},
		writer:  w,
		flusher: flusher,
	}

	// 创建请求上下文，如果请求有效
	respCtx.ReqCtx.IsSSE = true
	respCtx.IsSSE = true

	// Read and forward SSE events
	reader := bufio.NewReader(respCtx.Response.Body)

	// 如果启用了流量输出，初始化前缀并输出头部
	var dumpPrefix string
	if s.DumpTraffic {
		dumpPrefix = fmt.Sprintf("[DUMP] %s %s%s -> SSE Stream", respCtx.Response.Request.Method, respCtx.Response.Request.Host, respCtx.Response.Request.URL.RequestURI())

		// 输出响应状态行
		fmt.Printf("%s %s\n", dumpPrefix, respCtx.Response.Status)

		// 输出响应头部
		fmt.Printf("%s Response Headers:\n", dumpPrefix)
		for name, values := range respCtx.Response.Header {
			for _, value := range values {
				fmt.Printf("%s   %s: %s\n", dumpPrefix, name, value)
			}
		}

		fmt.Printf("%s Starting SSE stream\n", dumpPrefix)
	}

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			// 通知错误
			if respCtx.ReqCtx != nil {
				s.notifyError(fmt.Errorf("error reading SSE stream: %v", err), respCtx.ReqCtx)
			}
			return fmt.Errorf("error reading SSE stream: %v", err)
		}

		// 写入 tee，它会同时写入客户端和缓冲区
		_, err = tee.Write(line)
		if err != nil {
			// 通知错误
			if respCtx.ReqCtx != nil {
				s.notifyError(fmt.Errorf("error writing SSE data: %v", err), respCtx.ReqCtx)
			}
			return fmt.Errorf("error writing SSE data: %v", err)
		}

		// Log the event if verbose
		lineStr := strings.TrimSpace(string(line))
		logSSEEvent(lineStr, s.Verbose)

		// 如果启用了流量输出，输出 SSE 事件
		if s.DumpTraffic && lineStr != "" {
			fmt.Printf("%s %s\n", dumpPrefix, lineStr)
		}

		// 通知 SSE 事件处理
		if respCtx != nil && lineStr != "" {
			s.notifySSE(lineStr, respCtx)
		}
	}

	// 流结束后，记录 HAR 条目
	if s.HarLogger.IsEnabled() {
		// 计算流处理时间
		timeTaken := time.Since(startTime)
		if respCtx != nil {
			respCtx.TimeTaken = timeTaken
		}

		// 创建一个新的响应，包含收集到的完整数据
		newResp := &http.Response{
			Status:     respCtx.Response.Status,
			StatusCode: respCtx.Response.StatusCode,
			Header:     respCtx.Response.Header.Clone(),
			Body:       io.NopCloser(bytes.NewReader(tee.GetBuffer().Bytes())),
			Proto:      respCtx.Response.Proto,
			ProtoMajor: respCtx.Response.ProtoMajor,
			ProtoMinor: respCtx.Response.ProtoMinor,
		}

		// 使用原始请求记录 HAR 条目
		s.logToHAR(respCtx.Response.Request, newResp, startTime, timeTaken, false) // 这里使用 false 因为我们已经有了完整的数据

		if s.Verbose {
			log.Printf("[SSE] Recorded complete SSE response in HAR log (%d bytes)", tee.GetBuffer().Len())
		}
	}

	return nil
}

// logSSEEvent 记录 SSE 事件的日志
// 这个函数集中了所有 SSE 事件日志记录逻辑，避免代码重复
func logSSEEvent(lineStr string, verbose bool) {
	if !verbose || len(lineStr) <= 1 { // Skip empty lines or when verbose is disabled
		return
	}

	if strings.HasPrefix(lineStr, "data:") {
		log.Printf("[SSE] Event data: %s", lineStr)
	} else if strings.HasPrefix(lineStr, "event:") {
		log.Printf("[SSE] Event type: %s", lineStr)
	} else if strings.HasPrefix(lineStr, "id:") {
		log.Printf("[SSE] Event ID: %s", lineStr)
	} else if strings.HasPrefix(lineStr, "retry:") {
		log.Printf("[SSE] Event retry: %s", lineStr)
	} else if lineStr != "" {
		log.Printf("[SSE] Event line: %s", lineStr)
	}
}

// isServerSentEvent checks if the response is a Server-Sent Event stream
func isServerSentEvent(resp *http.Response) bool {
	// Check Content-Type header for SSE
	contentType := resp.Header.Get("Content-Type")

	// 检查是否是标准的 SSE Content-Type
	if strings.Contains(contentType, "text/event-stream") {
		return true
	}

	// 确保 Request 不为 nil
	if resp.Request == nil || resp.Request.URL == nil {
		return false
	}

	// 检查是否是 JSON 流
	// 注意：httpbin.org/stream 返回的是 JSON 流，而不是真正的 SSE
	// 但我们仍然可以将其作为流式处理
	if strings.Contains(contentType, "application/json") && strings.Contains(resp.Request.URL.Path, "/stream") {
		return true
	}

	// 检查是否是 OpenAI 的流式 API
	if strings.Contains(contentType, "application/json") && (strings.Contains(resp.Request.URL.Path, "/completions") ||
		strings.Contains(resp.Request.URL.Path, "/chat/completions")) {
		// 检查是否有 stream=true 参数
		if resp.Request.URL.Query().Get("stream") == "true" {
			return true
		}
	}

	return false
}

// isSSERequest checks if the request might be for a Server-Sent Event stream
func isSSERequest(req *http.Request) bool {
	// Check Accept header for SSE
	acceptHeader := req.Header.Get("Accept")

	// Check if the URL path contains common SSE endpoints
	path := strings.ToLower(req.URL.Path)

	// Common SSE endpoint patterns
	ssePatterns := []string{
		"/events",
		"/stream",
		"/sse",
		"/notifications",
		"/messages",
		"/updates",
		"/push",
		"/chat",
		"/completions",         // OpenAI API
		"/v1/chat/completions", // OpenAI API
	}

	// Check if the path contains any of the SSE patterns
	for _, pattern := range ssePatterns {
		if strings.Contains(path, pattern) {
			return true
		}
	}

	return strings.Contains(acceptHeader, "text/event-stream")
}

// mayBeServerSentEvent checks if the request might be for a Server-Sent Event stream
// This is used to set up the request properly before sending it
func mayBeServerSentEvent(req *http.Request) bool {
	// Check Accept header for SSE
	acceptHeader := req.Header.Get("Accept")
	return strings.Contains(acceptHeader, "text/event-stream")
}
