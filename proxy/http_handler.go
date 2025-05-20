package proxy

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

// handleHTTP is the handler for all incoming HTTP requests
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Received request: %s %s %s %s", r.Method, r.Host, r.URL.String(), r.Proto)

	if r.Method == http.MethodConnect {
		s.handleHTTPS(w, r)
		return
	}

	// Create a new request to the target server
	var targetURL string
	if r.URL.IsAbs() {
		targetURL = r.URL.String()
	} else {
		targetURL = "http://" + r.Host + r.URL.Path
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
	}

	// 构建transport
	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DisableCompression:    true,
		ResponseHeaderTimeout: 5 * time.Second,
	}

	s.handleProxyRequest(w, r, targetURL, baseTransport, false, nil)
}

// handleProxyRequest 统一处理HTTP和HTTPS的代理转发逻辑
func (s *Server) handleProxyRequest(
	w http.ResponseWriter,
	r *http.Request,
	targetURL string,
	baseTransport *http.Transport,
	isHTTPS bool,
	clientConn net.Conn, // 对于HTTPS 传递TLS连接，否则为nil
) {
	if s.UpstreamProxy != nil {
		if s.Verbose {
			log.Printf("[HTTP] Using upstream proxy: %s", s.UpstreamProxy.String())
		}
		baseTransport.Proxy = http.ProxyURL(s.UpstreamProxy)
	}
	s.handleHTTP2(baseTransport)
	transport := &earlySSEDetector{
		base:           baseTransport,
		responseWriter: w,
		server:         s,
		verbose:        s.Verbose,
	}

	startTime := time.Now()
	// 创建请求上下文
	reqCtx := s.createRequestContext(r, targetURL, startTime, isHTTPS)
	modifiedReq := s.notifyRequest(reqCtx)
	if modifiedReq != r && modifiedReq != nil {
		r = modifiedReq
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		log.Printf("[Proxy] Error creating proxy request for %s: %v", targetURL, err)
		if isHTTPS && clientConn != nil {
			clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		} else {
			http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		}
		s.notifyError(err, reqCtx)
		return
	}
	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}
	proxyReq.Host = r.Host

	potentialSSE := isSSERequest(proxyReq)
	if s.Verbose && potentialSSE {
		log.Printf("[Proxy] Potential SSE request detected based on URL path or Accept header")
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	if potentialSSE {
		client.Timeout = 0 // SSE无超时
		proxyReq.Header.Set("Accept", "text/event-stream")
		proxyReq.Header.Set("Cache-Control", "no-cache")
		proxyReq.Header.Set("Connection", "keep-alive")
	}

	resp, err := client.Do(proxyReq)
	timeTaken := time.Since(startTime)
	if err != nil {
		log.Printf("[Proxy] Error sending request to target server %s: %v", targetURL, err)
		if isHTTPS && clientConn != nil {
			clientConn.Write([]byte("HTTP/1.1 502 Bad Gateway\r\n\r\n"))
		} else {
			http.Error(w, "Error proxying to "+targetURL+": "+err.Error(), http.StatusBadGateway)
		}
		s.logToHAR(r, nil, startTime, timeTaken, false)
		s.notifyError(err, reqCtx)
		return
	}
	defer resp.Body.Close()
	if resp.Request == nil {
		resp.Request = proxyReq
	}

	s.processCompressedResponse(resp, reqCtx, s.Verbose)
	respCtx := s.createResponseContext(reqCtx, resp, timeTaken)
	modifiedResp := s.notifyResponse(respCtx)
	if modifiedResp != resp && modifiedResp != nil {
		resp = modifiedResp
		respCtx.Response = resp
	}
	if !isServerSentEvent(resp) {
		s.logToHAR(r, resp, startTime, timeTaken, false)
	}
	if s.DumpTraffic {
		s.dumpRequestBody(r)
		if !isServerSentEvent(resp) {
			s.dumpResponseBody(resp)
		}
	}
	if s.Verbose {
		log.Printf("[Proxy] Received response from %s: %d %s", targetURL, resp.StatusCode, resp.Status)
	} else {
		log.Printf("[Proxy] %s %s%s -> %d %s", r.Method, r.Host, r.URL.RequestURI(), resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	if isServerSentEvent(resp) {
		if s.Verbose {
			log.Printf("[Proxy] Detected Server-Sent Events response from %s", targetURL)
		}
		if isHTTPS && clientConn != nil {
			// HTTPS: 需要手动写入TLS连接
			writer := bufio.NewWriter(clientConn)
			statusLine := fmt.Sprintf("HTTP/%d.%d %d %s\r\n", resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.Status)
			writer.WriteString(statusLine)
			for k, vv := range resp.Header {
				for _, v := range vv {
					writer.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
				}
			}
			writer.WriteString("\r\n")
			writer.Flush()
			reader := bufio.NewReader(resp.Body)
			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					break
				}
				_, err = writer.Write(line)
				if err != nil {
					break
				}
				writer.Flush()
			}
			return
		} else {
			// HTTP明文: 直接用handleSSE
			err := s.handleSSE(w, respCtx)
			if err != nil {
				log.Printf("[SSE] Error handling SSE response: %v", err)
				s.notifyError(err, reqCtx)
			}
			return
		}
	}
	// 非SSE响应
	if isHTTPS && clientConn != nil {
		err := s.tunnelHTTPSResponse(clientConn.(*tls.Conn), resp, reqCtx)
		if err != nil {
			log.Printf("[Proxy] Error tunneling response to client: %v", err)
		}
		return
	}
	// HTTP明文: 直接写入w
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// 添加协议版本头以便前端识别
	w.Header().Add("X-Protocol", r.Proto)

	w.WriteHeader(resp.StatusCode)

	// 使用通用的流式传输函数处理响应
	contentType := resp.Header.Get("Content-Type")
	_, err = s.streamResponse(resp.Body, w, contentType, s.Verbose)
	if err != nil {
		log.Printf("[Proxy] Error streaming response: %v", err)
	}
}

// isTextContentType 判断Content-Type是否为文本类型
func isTextContentType(contentType string) bool {
	if contentType == "" {
		return false
	}

	contentType = strings.ToLower(contentType)

	// 移除可能的字符集和其他参数
	if idx := strings.Index(contentType, ";"); idx >= 0 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(contentType)

	// 直接匹配的类型
	knownTextTypes := []string{
		"text/",                             // 所有text/类型
		"application/json",                  // JSON
		"application/xml",                   // XML
		"application/javascript",            // JavaScript
		"application/x-javascript",          // 旧式JavaScript
		"application/ecmascript",            // ECMAScript
		"application/x-www-form-urlencoded", // 表单数据
		"application/xhtml+xml",             // XHTML
		"application/atom+xml",              // Atom
		"application/rss+xml",               // RSS
		"application/soap+xml",              // SOAP
		"application/x-yaml",                // YAML
		"application/yaml",                  // YAML
		"application/graphql",               // GraphQL
		"message/rfc822",                    // 邮件格式
	}

	for _, textType := range knownTextTypes {
		if strings.HasPrefix(contentType, textType) {
			return true
		}
	}

	// 包含特定后缀的类型
	knownTextSuffixes := []string{
		"+json", // JSON类型的变体如application/ld+json
		"+xml",  // XML类型的变体如application/rdf+xml
		"+text", // 任何带text后缀的类型
	}

	for _, suffix := range knownTextSuffixes {
		if strings.HasSuffix(contentType, suffix) {
			return true
		}
	}

	// 特定的不常见但仍是文本的MIME类型
	otherTextTypes := map[string]bool{
		"application/json-patch+json":  true,
		"application/merge-patch+json": true,
		"application/schema+json":      true,
		"application/vnd.api+json":     true,
		"application/vnd.github+json":  true,
		"application/problem+json":     true,
		"application/x-httpd-php":      true,
		"application/x-sh":             true,
		"application/x-csh":            true,
		"application/typescript":       true,
		"application/sql":              true,
		"application/csv":              true,
		"application/x-csv":            true,
		"text/csv":                     true,
		"application/ld+json":          true,
	}
	return otherTextTypes[contentType]
}
