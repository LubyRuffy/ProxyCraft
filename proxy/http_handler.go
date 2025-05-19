package proxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"
)

// handleHTTP is the handler for all incoming HTTP requests
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Verbose {
		log.Printf("[HTTP] Received request: %s %s %s %s", r.Method, r.Host, r.URL.String(), r.Proto)
	} else {
		log.Printf("[HTTP] %s %s%s", r.Method, r.Host, r.URL.RequestURI())
	}

	if r.Method == http.MethodConnect {
		s.handleHTTPS(w, r)
		return
	}

	// Create a new request to the target server
	var targetURL string
	if r.URL.IsAbs() {
		targetURL = r.URL.String()
	} else {
		// If URL is not absolute, scheme is http by default for proxy requests unless it's CONNECT
		// For non-CONNECT, r.Host contains the target host and port
		targetURL = "http://" + r.Host + r.URL.Path
		if r.URL.RawQuery != "" {
			targetURL += "?" + r.URL.RawQuery
		}
	}

	if s.Verbose {
		log.Printf("[HTTP] Forwarding request to: %s %s", r.Method, targetURL)
	}

	// 创建请求上下文
	startTime := time.Now()
	reqCtx := s.createRequestContext(r, targetURL, startTime, false)

	// 通知请求事件
	modifiedReq := s.notifyRequest(reqCtx)
	if modifiedReq != r && modifiedReq != nil {
		r = modifiedReq
	}

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		log.Printf("[HTTP] Error creating proxy request for %s: %v", targetURL, err)
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		s.notifyError(err, reqCtx)
		return
	}

	// Copy headers from original request to proxy request
	for k, vv := range r.Header {
		for _, v := range vv {
			proxyReq.Header.Add(k, v)
		}
	}
	// Ensure Host header is set correctly for the target server
	proxyReq.Host = r.Host

	// Check if this might be an SSE request based on patterns and headers
	potentialSSE := isSSERequest(proxyReq)
	if s.Verbose && potentialSSE {
		log.Printf("[HTTP] Potential SSE request detected based on URL path or Accept header")
	}

	// Create a custom RoundTripper that can intercept SSE responses early
	baseTransport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		// Don't automatically decompress responses to allow proper SSE handling
		DisableCompression: true,
		// Override the RoundTrip method to intercept responses early
		ResponseHeaderTimeout: 5 * time.Second, // Timeout for receiving response headers
	}

	// 如果配置了上层代理，设置代理URL
	if s.UpstreamProxy != nil {
		if s.Verbose {
			log.Printf("[HTTP] Using upstream proxy: %s", s.UpstreamProxy.String())
		}
		baseTransport.Proxy = http.ProxyURL(s.UpstreamProxy)
	}

	// Configure HTTP/2 support
	s.handleHTTP2(baseTransport)

	// Create a custom transport that can detect SSE responses early
	transport := &earlySSEDetector{
		base:           baseTransport,
		responseWriter: w,
		server:         s,
		verbose:        s.Verbose,
	}

	// Special handling for potential SSE requests
	if potentialSSE {
		if s.Verbose {
			log.Printf("[HTTP] Using special SSE handling for %s", targetURL)
		}

		// Use a custom client with no timeout for SSE
		client := &http.Client{
			Transport: transport,
			// No timeout for SSE requests
		}

		// Set special headers for SSE
		proxyReq.Header.Set("Accept", "text/event-stream")
		proxyReq.Header.Set("Cache-Control", "no-cache")
		proxyReq.Header.Set("Connection", "keep-alive")

		// Send the request
		resp, err := client.Do(proxyReq)
		timeTaken := time.Since(startTime)

		if err != nil {
			log.Printf("[HTTP] Error sending request to target server %s: %v", targetURL, err)
			http.Error(w, fmt.Sprintf("Error proxying to %s: %v", targetURL, err), http.StatusBadGateway)
			s.notifyError(err, reqCtx)
			return
		}
		defer resp.Body.Close()

		if resp.Request == nil {
			resp.Request = proxyReq
		}

		// 创建响应上下文
		respCtx := s.createResponseContext(reqCtx, resp, timeTaken)

		// 通知响应事件
		modifiedResp := s.notifyResponse(respCtx)
		if modifiedResp != resp && modifiedResp != nil {
			resp = modifiedResp
			respCtx.Response = resp
		}

		// Log to HAR - 但对于SSE响应，我们在 handleSSE 中记录
		// 只为非 SSE 响应记录 HAR 条目
		if !isServerSentEvent(resp) && s.HarLogger.IsEnabled() {
			serverIP := ""
			if proxyReq != nil && proxyReq.URL != nil {
				serverIP = proxyReq.URL.Host
			}
			s.HarLogger.AddEntry(r, resp, startTime, timeTaken, serverIP, r.RemoteAddr)
		}

		// Check if this is actually an SSE response
		if isServerSentEvent(resp) {
			if s.Verbose {
				log.Printf("[HTTP] Confirmed SSE response from %s", targetURL)
			}

			// Handle SSE response
			err := s.handleSSE(w, resp)
			if err != nil {
				log.Printf("[SSE] Error handling SSE response: %v", err)
				s.notifyError(err, reqCtx)
			}
			return
		} else {
			// Not an SSE response, handle normally
			if s.Verbose {
				log.Printf("[HTTP] Expected SSE but got %s from %s", resp.Header.Get("Content-Type"), targetURL)
			}

			// Copy headers from target server's response to our response writer
			for k, vv := range resp.Header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}

			// Set the status code
			w.WriteHeader(resp.StatusCode)

			// Copy the body from target server's response to our response writer
			written, err := io.Copy(w, resp.Body)
			if err != nil {
				log.Printf("Error copying response body: %v", err)
				s.notifyError(err, reqCtx)
			}

			log.Printf("Copied %d bytes for response body from %s", written, targetURL)
			return
		}
	}

	// For non-SSE requests, use normal handling
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	resp, err := client.Do(proxyReq)
	timeTaken := time.Since(startTime)

	if err != nil {
		log.Printf("[HTTP] Error sending request to target server %s: %v", targetURL, err)
		http.Error(w, fmt.Sprintf("Error proxying to %s: %v", targetURL, err), http.StatusBadGateway)
		// Log to HAR even if there's an error sending the request (resp might be nil)
		s.logToHAR(r, nil, startTime, timeTaken, false)
		s.notifyError(err, reqCtx)
		return
	}
	defer resp.Body.Close()

	if resp.Request == nil {
		resp.Request = proxyReq
	}

	// 创建响应上下文
	respCtx := s.createResponseContext(reqCtx, resp, timeTaken)

	// 通知响应事件
	modifiedResp := s.notifyResponse(respCtx)
	if modifiedResp != resp && modifiedResp != nil {
		resp = modifiedResp
		respCtx.Response = resp
	}

	// Log to HAR - 但对于SSE响应，我们在 handleSSE 中记录
	// 只为非 SSE 响应记录 HAR 条目
	if !isServerSentEvent(resp) {
		s.logToHAR(r, resp, startTime, timeTaken, false)
	}

	// 如果启用了流量输出，输出请求和响应内容
	if s.DumpTraffic {
		s.dumpRequestBody(r)
		if !isServerSentEvent(resp) { // SSE响应在handleSSE中处理
			s.dumpResponseBody(resp)
		}
	}

	if s.Verbose {
		log.Printf("[HTTP] Received response from %s: %d %s", targetURL, resp.StatusCode, resp.Status)
	} else {
		log.Printf("[HTTP] %s %s%s -> %d %s", r.Method, r.Host, r.URL.RequestURI(), resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	// Check if this is a Server-Sent Events response
	if isServerSentEvent(resp) {
		if s.Verbose {
			log.Printf("[HTTP] Detected Server-Sent Events response from %s", targetURL)
		}

		// Handle SSE response
		err := s.handleSSE(w, resp)
		if err != nil {
			log.Printf("[SSE] Error handling SSE response: %v", err)
			s.notifyError(err, reqCtx)
		}
		return
	}

	// For non-SSE responses, proceed with normal handling
	// Copy headers from target server's response to our response writer
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// Set the status code
	w.WriteHeader(resp.StatusCode)

	// Copy the body from target server's response to our response writer
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
		s.notifyError(err, reqCtx)
		// Don't send http.Error here as headers might have already been written
	}

	// Log protocol information if verbose
	if s.Verbose {
		proto := "HTTP/1.1"
		if resp.ProtoMajor == 2 {
			proto = "HTTP/2.0"
		}
		log.Printf("[HTTP] Response protocol: %s", proto)
	}

	log.Printf("Copied %d bytes for response body from %s", written, targetURL)
}
