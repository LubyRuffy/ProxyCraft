package proxy

import (
	"bytes"
	"compress/flate"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"
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

				// 我们不再在这里开始处理SSE事件，只设置适当的头部
				// 将完整的事件处理交给handleSSE函数

				// 设置正确的头部以确保正确的流传输
				resp.Header.Set("Content-Type", "text/event-stream")
				resp.Header.Set("Cache-Control", "no-cache")
				resp.Header.Set("Connection", "keep-alive")

				// 不再创建管道和新响应体，只标记响应是SSE
				return resp, nil
			}

			// 对于非SSE响应，返回原始响应
			return resp, nil
		},
	}

	// 使用自定义Transport执行请求
	return transport.RoundTrip(req)
}

// isBinaryContent 检查数据是否是二进制格式（而非文本）
func isBinaryContent(data []byte, contentType string) bool {
	// 首先基于Content-Type检查
	if contentType != "" {
		contentTypeLower := strings.ToLower(contentType)

		// 移除参数部分
		if idx := strings.Index(contentTypeLower, ";"); idx >= 0 {
			contentTypeLower = contentTypeLower[:idx]
		}
		contentTypeLower = strings.TrimSpace(contentTypeLower)

		// 已知文本类型，不用检查内容直接返回false
		if isTextContentType(contentType) {
			// 如果数据很短，仍需检查内容
			if len(data) < 32 {
				// 短数据需要通过实际内容来判断
			} else {
				return false
			}
		}

		// 已知二进制类型，不用检查内容直接返回true
		binaryPrefixes := []string{
			"image/",
			"audio/",
			"video/",
			"application/octet-stream",
			"application/pdf",
			"application/zip",
			"application/x-gzip",
			"application/x-tar",
			"application/x-7z-compressed",
			"application/x-rar-compressed",
			"application/x-msdownload",
			"application/vnd.ms-",
			"application/vnd.openxmlformats-",
			"font/",
			"model/",
		}

		for _, prefix := range binaryPrefixes {
			if strings.HasPrefix(contentTypeLower, prefix) {
				return true
			}
		}
	}

	// 如果没有数据，无法确定
	if len(data) == 0 {
		return false
	}

	// 检查是否是有效的UTF-8文本
	if utf8.Valid(data) {
		// 对于有效的UTF-8文本，还需检查是否包含过多控制字符
		controlCount := 0
		totalCount := 0
		maxSamples := 1024 // 最多检查前1024个字节

		// 计算要检查的字节数
		bytesToCheck := len(data)
		if bytesToCheck > maxSamples {
			bytesToCheck = maxSamples
		}

		for i := 0; i < bytesToCheck; i++ {
			b := data[i]
			totalCount++

			// 控制字符 (ASCII 0-31，除了常见的9=tab, 10=LF, 13=CR)
			if b < 32 && b != 9 && b != 10 && b != 13 {
				controlCount++
			}
		}

		// 如果控制字符过多，可能是二进制
		if float64(controlCount)/float64(totalCount) > 0.15 { // 超过15%是控制字符
			return true
		}

		// 是有效的UTF-8文本且控制字符不多，判定为文本
		return false
	}

	// 不是有效的UTF-8文本，判定为二进制
	return true
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

// dumpResponseBody 输出响应体内容
func (s *Server) dumpResponseBody(resp *http.Response) {
	if !s.DumpTraffic {
		return
	}

	if resp == nil || resp.Body == nil {
		return
	}

	fmt.Println(strings.Repeat("<", 20))
	defer fmt.Println(strings.Repeat("<", 20))

	// 输出响应行
	fmt.Printf("%s %s\n", resp.Proto, resp.Status)

	// 输出响应头部
	for name, values := range resp.Header {
		for _, value := range values {
			fmt.Printf("%s: %s\n", name, value)
		}
	}
	fmt.Println()

	// 获取内容类型和编码
	contentType := resp.Header.Get("Content-Type")
	contentEncoding := resp.Header.Get("Content-Encoding")

	// 创建响应的副本，避免修改原始响应
	respCopy := *resp
	respCopy.Body = resp.Body

	// 如果响应体被压缩，先进行解压
	if contentEncoding != "" {
		if err := decompressBody(&respCopy); err != nil {
			log.Printf("解压响应体失败: %v", err)
			// 添加提示信息
			fmt.Printf("(压缩内容解析失败，显示原始数据，编码: %s)\n", contentEncoding)
			// 即使解压失败，仍然继续尝试读取原始内容
		} else {
			// 添加提示信息
			fmt.Printf("(已自动解压 %s 编码的内容)\n", contentEncoding)
		}
	}

	// 读取响应体（可能是已解压的内容）
	bodyBytes, err := readAndRestoreBody(&respCopy.Body, respCopy.ContentLength)
	if err != nil {
		log.Printf("读取响应体失败: %v", err)
		return
	}

	// 恢复原始响应的Body
	resp.Body = respCopy.Body

	// 没有内容直接返回
	if len(bodyBytes) == 0 {
		fmt.Println("(empty body)")
		return
	}

	// 检查是否为二进制内容
	if isBinaryContent(bodyBytes, contentType) {
		fmt.Printf("(binary data, %d bytes)\n", len(bodyBytes))
		return
	}

	// 显示文本内容
	fmt.Println(string(bodyBytes))
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

// decompressBody 解压响应体，根据Content-Encoding头部支持gzip, deflate等压缩格式
func decompressBody(resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}

	// 获取内容编码
	contentEncoding := resp.Header.Get("Content-Encoding")
	if contentEncoding == "" {
		return nil // 没有编码，不需要解压
	}

	// 读取原始响应体
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("读取原始响应体失败: %w", err)
	}
	resp.Body.Close()

	// 如果内容为空，无需解压
	if len(bodyBytes) == 0 {
		resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return nil
	}

	// 保存原始数据的副本，以便在解压失败时恢复
	originalBodyBytes := make([]byte, len(bodyBytes))
	copy(originalBodyBytes, bodyBytes)

	// 解压数据
	decompressedBytes, err := decompressData(bodyBytes, contentEncoding)
	if err != nil {
		// 解压失败，恢复原始响应体
		resp.Body = io.NopCloser(bytes.NewReader(originalBodyBytes))
		return fmt.Errorf("解压响应体失败: %w", err)
	}

	// 更新响应头
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", fmt.Sprint(len(decompressedBytes)))

	// 替换响应体
	resp.Body = io.NopCloser(bytes.NewReader(decompressedBytes))
	resp.ContentLength = int64(len(decompressedBytes))
	resp.Uncompressed = true

	// 移除分块传输编码标记
	resp.TransferEncoding = nil

	return nil
}

// decompressData 根据指定的编码方式解压数据
func decompressData(data []byte, encoding string) ([]byte, error) {
	if len(data) == 0 {
		return data, nil
	}

	// 规范化编码名称
	encoding = strings.ToLower(strings.TrimSpace(encoding))

	// 处理多种编码格式，例如 "gzip, deflate"
	encodings := strings.Split(encoding, ",")
	result := data

	// 从最外层的编码开始处理（最右边的编码是最外层的）
	for i := len(encodings) - 1; i >= 0; i-- {
		enc := strings.TrimSpace(encodings[i])

		switch enc {
		case "gzip":
			// 检查gzip魔术数字
			if len(result) < 2 || result[0] != 0x1f || result[1] != 0x8b {
				// 不是有效的gzip数据
				return nil, fmt.Errorf("无效的gzip数据: 缺少正确的魔术数字")
			}

			reader, err := gzip.NewReader(bytes.NewReader(result))
			if err != nil {
				return nil, fmt.Errorf("创建gzip解压器失败: %w", err)
			}

			defer reader.Close()
			decompressed, err := io.ReadAll(reader)
			if err != nil {
				return nil, fmt.Errorf("读取gzip解压数据失败: %w", err)
			}

			result = decompressed

		case "deflate":
			// deflate没有明确的魔术数字，但我们可以尝试解压
			reader := flate.NewReader(bytes.NewReader(result))
			defer reader.Close()

			decompressed, err := io.ReadAll(reader)
			if err != nil {
				// 尝试旧式deflate（不带zlib头部）
				// 某些服务器可能发送的是原始deflate数据而非zlib格式的deflate数据
				r := flate.NewReader(bytes.NewReader(result))
				defer r.Close()

				decompressed, err = io.ReadAll(r)
				if err != nil {
					return nil, fmt.Errorf("读取deflate解压数据失败: %w", err)
				}
			}

			result = decompressed

		case "br":
			// brotli压缩，目前不支持
			// 需要添加brotli库支持
			return nil, fmt.Errorf("不支持的编码方式: %s", enc)

		case "identity":
			// 不做任何处理
			continue

		default:
			// 不支持的编码
			return nil, fmt.Errorf("不支持的编码方式: %s", enc)
		}

		// 检查解压后的数据是否为空
		if len(result) == 0 {
			return nil, fmt.Errorf("%s解压后数据为空", enc)
		}
	}

	return result, nil
}

// handleCompressedResponse 已被processCompressedResponse替代

// processCompressedResponse 处理压缩响应体，包括解压缩和处理Content-Encoding/Content-Length头
// 这是一个更简单的辅助函数，专注于处理压缩，而不涉及上下文创建和事件通知
func (s *Server) processCompressedResponse(resp *http.Response, reqCtx *RequestContext, verbose bool) {
	// 先检查是否是SSE响应，如果是则跳过解压步骤
	if resp != nil && isServerSentEvent(resp) {
		if verbose {
			log.Printf("[HTTP] 检测到SSE响应，跳过解压缩处理以保持流式传输")
		}
		return
	}

	// 检查和处理压缩响应
	isCompressed := resp != nil &&
		isTextContentType(resp.Header.Get("Content-Type")) &&
		resp.Header.Get("Content-Encoding") != ""

	if isCompressed {
		if verbose {
			log.Printf("[HTTP] 检测到压缩的文本内容: %s, 编码: %s",
				resp.Header.Get("Content-Type"),
				resp.Header.Get("Content-Encoding"))
		}

		err := decompressBody(resp)
		if err != nil {
			log.Printf("[HTTP] 解压响应体失败: %v", err)
			if reqCtx != nil {
				s.notifyError(err, reqCtx)
			}
		} else if verbose {
			log.Printf("[HTTP] 成功解压响应体")
		}
	}
}
