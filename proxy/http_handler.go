package proxy

import (
	"log"
	"net/http"
	"strings"
)

// handleHTTP is the handler for all incoming HTTP requests
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("[HTTP] Received request: %s %s %s %s", r.Method, r.Host, r.URL.String(), r.Proto)

	if r.Method == http.MethodConnect {
		s.handleHTTPS(w, r)
		return
	}

	targetURL := s.resolveTargetURL(r)

	baseTransport := s.newTransport(r.Host, false)
	transport := s.wrapTransportForSSE(baseTransport)

	proxyReq, reqCtx, potentialSSE, startTime, err := s.prepareProxyRequest(r, targetURL, false)
	if err != nil {
		log.Printf("[Proxy] Error creating proxy request for %s: %v", targetURL, err)
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	logPotentialSSE(s.Verbose, "[Proxy]", potentialSSE)

	resp, timeTaken, err := s.sendProxyRequest(proxyReq, transport, potentialSSE, startTime)
	if err != nil {
		log.Printf("[Proxy] Error sending request to target server %s: %v", targetURL, err)
		s.recordProxyError(err, reqCtx, startTime, timeTaken)
		http.Error(w, "Error proxying to "+targetURL+": "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respCtx, isSSE := s.processProxyResponse(reqCtx, resp, startTime, timeTaken, "[Proxy]", targetURL)

	if isSSE {
		if err := s.handleSSE(w, respCtx); err != nil {
			log.Printf("[SSE] Error handling SSE response: %v", err)
			s.notifyError(err, reqCtx)
		}
		return
	}

	if err := s.writeHTTPResponse(w, respCtx, r.Proto); err != nil {
		log.Printf("[Proxy] Error streaming response: %v", err)
	}
}

// resolveTargetURL builds the absolute target URL for the incoming request.
func (s *Server) resolveTargetURL(r *http.Request) string {
	if r.URL.IsAbs() {
		return r.URL.String()
	}

	targetURL := "http://" + r.Host + r.URL.Path
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}
	return targetURL
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
