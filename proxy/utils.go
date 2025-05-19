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

// logToHAR 是一个辅助方法，用于统一处理 HAR 日志记录
// 这个方法集中了所有 HAR 日志记录逻辑，避免代码重复
func (s *Server) logToHAR(req *http.Request, resp *http.Response, startTime time.Time, timeTaken time.Duration, isSSE bool) {
	if s.HarLogger == nil || !s.HarLogger.IsEnabled() {
		return
	}

	// 获取服务器 IP 和连接 ID
	serverIP := ""
	connectionID := ""

	// 从请求中获取服务器 IP
	if req != nil {
		connectionID = req.RemoteAddr
		if req.URL != nil {
			serverIP = req.URL.Host
		}
	}

	// 对于 SSE 响应，创建一个没有响应体的副本，以避免读取整个响应体
	if isSSE && resp != nil {
		respCopy := *resp
		respCopy.Body = nil
		s.HarLogger.AddEntry(req, &respCopy, startTime, timeTaken, serverIP, connectionID)
	} else {
		// 对于非 SSE 响应或错误情况，正常记录
		s.HarLogger.AddEntry(req, resp, startTime, timeTaken, serverIP, connectionID)
	}
}

// createRequestContext 创建一个请求上下文
func (s *Server) createRequestContext(req *http.Request, targetURL string, startTime time.Time, isHTTPS bool) *RequestContext {
	return &RequestContext{
		Request:   req,
		StartTime: startTime,
		IsSSE:     isSSERequest(req),
		IsHTTPS:   isHTTPS,
		TargetURL: targetURL,
		UserData:  make(map[string]interface{}),
	}
}

// createResponseContext 创建一个响应上下文
func (s *Server) createResponseContext(reqCtx *RequestContext, resp *http.Response, timeTaken time.Duration) *ResponseContext {
	return &ResponseContext{
		ReqCtx:    reqCtx,
		Response:  resp,
		TimeTaken: timeTaken,
		IsSSE:     isServerSentEvent(resp),
		UserData:  make(map[string]interface{}),
	}
}

// notifyRequest 通知请求事件
func (s *Server) notifyRequest(ctx *RequestContext) *http.Request {
	if s.EventHandler != nil {
		return s.EventHandler.OnRequest(ctx)
	}
	return ctx.Request
}

// notifyResponse 通知响应事件
func (s *Server) notifyResponse(ctx *ResponseContext) *http.Response {
	if s.EventHandler != nil {
		return s.EventHandler.OnResponse(ctx)
	}
	return ctx.Response
}

// notifyError 通知错误事件
func (s *Server) notifyError(err error, reqCtx *RequestContext) {
	if s.EventHandler != nil {
		s.EventHandler.OnError(err, reqCtx)
	}
}

// notifyTunnelEstablished 通知隧道建立事件
func (s *Server) notifyTunnelEstablished(host string, isIntercepted bool) {
	if s.EventHandler != nil {
		s.EventHandler.OnTunnelEstablished(host, isIntercepted)
	}
}

// notifySSE 通知SSE事件
func (s *Server) notifySSE(event string, ctx *ResponseContext) {
	if s.EventHandler != nil {
		s.EventHandler.OnSSE(event, ctx)
	}
}

// headerInterceptingTransport 是一个自定义的 http.RoundTripper，它可以在接收到响应头后立即拦截响应
type headerInterceptingTransport struct {
	base     http.RoundTripper
	verbose  bool
	callback func(*http.Response) (*http.Response, error)
}

// RoundTrip 实现 http.RoundTripper 接口
func (t *headerInterceptingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// 使用基础 Transport 执行请求
	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// 在收到响应头后立即调用回调函数
	if t.callback != nil {
		return t.callback(resp)
	}

	// 如果没有回调函数，返回原始响应
	return resp, nil
}

// earlySSEDetector is a custom http.RoundTripper that can detect and handle SSE responses
// immediately after receiving response headers, before any of the response body is read
type earlySSEDetector struct {
	base           http.RoundTripper
	responseWriter interface{} // Can be http.ResponseWriter or *tls.Conn
	server         *Server
	verbose        bool
}

// RoundTrip implements the http.RoundTripper interface
func (t *earlySSEDetector) RoundTrip(req *http.Request) (*http.Response, error) {
	// 创建一个自定义的Transport，它可以拦截响应头
	transport := &headerInterceptingTransport{
		base:    t.base,
		verbose: t.verbose,
		callback: func(resp *http.Response) (*http.Response, error) {
			// 检查是否是SSE响应
			if isServerSentEvent(resp) {
				if t.verbose {
					log.Printf("[SSE] Detected SSE response early based on Content-Type header")
				}

				// 对于SSE响应，创建一个管道来流式传输数据
				pr, pw := io.Pipe()

				// 创建一个新的响应，使用管道读取器作为响应体
				newResp := &http.Response{
					Status:        resp.Status,
					StatusCode:    resp.StatusCode,
					Header:        resp.Header.Clone(),
					Body:          pr, // 使用管道读取器作为新的响应体
					ContentLength: -1, // 未知长度，用于流式传输
					Proto:         resp.Proto,
					ProtoMajor:    resp.ProtoMajor,
					ProtoMinor:    resp.ProtoMinor,
				}

				// 确保为SSE流设置关键头部
				newResp.Header.Set("Content-Type", "text/event-stream")
				newResp.Header.Set("Cache-Control", "no-cache")
				newResp.Header.Set("Connection", "keep-alive")
				newResp.Header.Set("Transfer-Encoding", "chunked")

				// 启动一个goroutine从原始响应中读取并写入我们的管道
				go func() {
					defer resp.Body.Close()
					defer pw.Close()

					// 为原始响应体创建一个读取器
					reader := bufio.NewReader(resp.Body)

					// 读取并转发每一行
					for {
						line, err := reader.ReadBytes('\n')
						if err != nil {
							if err == io.EOF {
								break
							}
							log.Printf("[SSE] Error reading SSE stream: %v", err)
							break
						}

						// 将行写入我们的管道
						_, err = pw.Write(line)
						if err != nil {
							log.Printf("[SSE] Error writing to pipe: %v", err)
							break
						}

						// 如果启用了详细模式，记录事件
						lineStr := strings.TrimSpace(string(line))
						logSSEEvent(lineStr, t.verbose)
					}

					if t.verbose {
						log.Printf("[SSE] Finished streaming SSE response")
					}
				}()

				// 返回带有管道读取器作为响应体的新响应
				return newResp, nil
			}

			// 对于非SSE响应，只返回原始响应
			return resp, nil
		},
	}

	// 使用自定义Transport执行请求
	return transport.RoundTrip(req)
}

// isBinaryContent 检查内容是否为二进制
func isBinaryContent(data []byte, contentType string) bool {
	// 首先检查 Content-Type
	if strings.HasPrefix(contentType, "image/") ||
		strings.HasPrefix(contentType, "audio/") ||
		strings.HasPrefix(contentType, "video/") ||
		strings.HasPrefix(contentType, "application/octet-stream") ||
		strings.HasPrefix(contentType, "application/pdf") ||
		strings.HasPrefix(contentType, "application/zip") ||
		strings.HasPrefix(contentType, "application/x-gzip") ||
		strings.Contains(contentType, "compressed") ||
		strings.Contains(contentType, "binary") {
		return true
	}

	// 如果没有内容，不是二进制
	if len(data) == 0 {
		return false
	}

	// 检查数据中是否包含太多的非打印字符
	binaryCount := 0
	for i := 0; i < len(data) && i < 512; i++ { // 只检查前512字节
		if data[i] < 7 || (data[i] > 14 && data[i] < 32) || data[i] > 127 {
			binaryCount++
		}
	}

	// 如果非打印字符超过一定比例，认为是二进制
	return float64(binaryCount)/float64(min(len(data), 512)) > 0.1
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// readAndRestoreBody 读取并恢复请求/响应体
func readAndRestoreBody(bodySlot *io.ReadCloser, contentLength int64) ([]byte, error) {
	if bodySlot == nil || *bodySlot == nil || *bodySlot == http.NoBody {
		return nil, nil
	}

	// 读取全部内容
	bodyBytes, err := io.ReadAll(*bodySlot)
	_ = (*bodySlot).Close() // 关闭原始体

	if err != nil {
		// 出错时，将体替换为空读取器以防止进一步错误
		*bodySlot = io.NopCloser(strings.NewReader("")) // 出错时设置为空读取器
		return nil, err
	}

	*bodySlot = io.NopCloser(bytes.NewBuffer(bodyBytes)) // 恢复体
	return bodyBytes, nil
}

// dumpRequestBody 输出请求头部和体内容
func (s *Server) dumpRequestBody(req *http.Request) {
	if !s.DumpTraffic {
		return
	}

	fmt.Println(strings.Repeat(">", 20))
	defer fmt.Println(strings.Repeat(">", 20))

	// 输出请求行
	fmt.Printf("%s %s %s\n", req.Method, req.URL, req.Proto)

	// 输出请求头部
	for name, values := range req.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}

	// 读取并恢复请求体
	bodyBytes, err := readAndRestoreBody(&req.Body, req.ContentLength)
	if err != nil {
		log.Printf("Error reading request body for dump: %v\n", err)
		return
	}

	// 检查是否为二进制内容
	contentType := req.Header.Get("Content-Type")
	if isBinaryContent(bodyBytes, contentType) {
		log.Printf("Binary request body detected (%d bytes), not displaying\n", len(bodyBytes))
		fmt.Println("\n(binary data)")
		return
	}

	// 输出文本内容
	if len(bodyBytes) > 0 {
		fmt.Printf("\n%s\n", string(bodyBytes))
	}
}

// dumpResponseBody 输出响应头部和体内容
func (s *Server) dumpResponseBody(resp *http.Response) {
	if !s.DumpTraffic || resp == nil {
		return
	}

	fmt.Println(strings.Repeat("<", 20))
	defer fmt.Println(strings.Repeat("<", 20))

	// 输出响应状态行
	fmt.Printf("%s %s\n", resp.Proto, resp.Status)

	// 输出响应头部
	for name, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}

	// 读取并恢复响应体
	bodyBytes, err := readAndRestoreBody(&resp.Body, resp.ContentLength)
	if err != nil {
		log.Printf("Error reading response body for dump: %v\n", err)
		return
	}

	// 检查是否为二进制内容
	contentType := resp.Header.Get("Content-Type")
	if isBinaryContent(bodyBytes, contentType) {
		log.Printf("Binary response body detected (%d bytes), not displaying\n", len(bodyBytes))
		fmt.Println("\n(binary data)")
		return
	}

	// 输出文本内容
	if len(bodyBytes) > 0 {
		fmt.Printf("\n%s\n", string(bodyBytes))
	}
}

// logHeader 用于记录HTTP头部信息
func logHeader(header http.Header, prefix string) {
	fmt.Println(prefix)
	for k, vv := range header {
		for _, v := range vv {
			fmt.Printf("  %s: %s\n", k, v)
		}
	}
}
