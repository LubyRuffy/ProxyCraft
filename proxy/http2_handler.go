package proxy

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/http2"
)

// handleHTTP2 configures HTTP/2 support for client and server connections
func (s *Server) handleHTTP2(transport *http.Transport) {
	// Configure HTTP/2 support for the transport
	err := http2.ConfigureTransport(transport)
	if err != nil {
		log.Printf("Error configuring HTTP/2 transport: %v", err)
		return
	}

	if s.Verbose {
		log.Printf("HTTP/2 support enabled for transport")
	}
}

// handleHTTP2MITM handles HTTP/2 connections in MITM mode
func (s *Server) handleHTTP2MITM(tlsConn *tls.Conn, connectReq *http.Request) {
	if s.Verbose {
		log.Printf("[HTTP/2] Handling HTTP/2 connection for %s", connectReq.Host)
	}

	// 通知隧道已建立
	s.notifyTunnelEstablished(connectReq.Host, true)

	// Create an HTTP/2 server
	server := &http2.Server{}

	// Create a connection wrapper
	conn := &http2MITMConn{
		server:      server,
		conn:        tlsConn,
		originalReq: connectReq,
		proxy:       s,
	}

	// Serve the connection
	server.ServeConn(tlsConn, &http2.ServeConnOpts{
		Handler: conn,
	})
}

// http2MITMConn is a connection wrapper for HTTP/2 MITM
type http2MITMConn struct {
	server      *http2.Server
	conn        *tls.Conn
	originalReq *http.Request
	proxy       *Server
}

// ServeHTTP implements http.Handler for the HTTP/2 connection
func (h *http2MITMConn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.proxy.Verbose {
		log.Printf("[HTTP/2] Received request: %s %s", r.Method, r.URL.String())
	} else {
		log.Printf("[HTTP/2] %s %s%s", r.Method, r.Host, r.URL.RequestURI())
	}

	// 检查conn是否为nil，这在测试中可能会发生
	if h.conn == nil {
		http.Error(w, "Connection is not available", http.StatusBadGateway)
		return
	}

	// Create a new request to the target server
	targetURL := &url.URL{
		Scheme:   "https",
		Host:     h.originalReq.Host,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	// 创建请求上下文
	startTime := time.Now()
	reqCtx := h.proxy.createRequestContext(r, targetURL.String(), startTime, true)

	// 通知请求事件
	modifiedReq := h.proxy.notifyRequest(reqCtx)
	if modifiedReq != r && modifiedReq != nil {
		r = modifiedReq
	}

	outReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		log.Printf("[HTTP/2] Error creating outgoing request: %v", err)
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		h.proxy.notifyError(err, reqCtx)
		return
	}

	// Copy headers from original request to proxy request
	for k, vv := range r.Header {
		for _, v := range vv {
			outReq.Header.Add(k, v)
		}
	}

	// Ensure Host header is set correctly for the target server
	outReq.Host = r.Host

	// Extract hostname without port for SNI
	targetHostname := h.originalReq.Host
	if h, _, err := net.SplitHostPort(h.originalReq.Host); err == nil {
		targetHostname = h
	}

	// Send the request to the target server
	startTime = time.Now()

	// Check if this might be an SSE request based on patterns and headers
	potentialSSE := isSSERequest(outReq)
	if h.proxy.Verbose && potentialSSE {
		log.Printf("[HTTP/2] Potential SSE request detected based on URL path or Accept header")
	}

	// Create a transport with HTTP/2 support
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true,           // Skip certificate verification
			ServerName:         targetHostname, // Set SNI (Server Name Indication)
		},
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
	}

	// 如果配置了上层代理，设置代理URL
	if h.proxy.UpstreamProxy != nil {
		if h.proxy.Verbose {
			log.Printf("[HTTP/2] Using upstream proxy: %s", h.proxy.UpstreamProxy.String())
		}
		transport.Proxy = http.ProxyURL(h.proxy.UpstreamProxy)
	}

	// Configure HTTP/2 support for the transport
	h.proxy.handleHTTP2(transport)

	// Create a custom transport that can detect SSE responses early
	sseTransport := &earlySSEDetector{
		base:           transport,
		responseWriter: w,
		server:         h.proxy,
		verbose:        h.proxy.Verbose,
	}

	// Special handling for potential SSE requests
	if potentialSSE {
		if h.proxy.Verbose {
			log.Printf("[HTTP/2] Using special SSE handling for %s", targetURL.String())
		}

		// Use a custom client with no timeout for SSE
		client := &http.Client{
			Transport: sseTransport,
			// No timeout for SSE requests
		}

		// Set special headers for SSE
		outReq.Header.Set("Accept", "text/event-stream")
		outReq.Header.Set("Cache-Control", "no-cache")
		outReq.Header.Set("Connection", "keep-alive")

		// Send the request
		resp, err := client.Do(outReq)
		timeTaken := time.Since(startTime)

		if err != nil {
			log.Printf("[HTTP/2] Error sending request to target server %s: %v", targetURL.String(), err)
			http.Error(w, fmt.Sprintf("Error proxying to %s: %v", targetURL.String(), err), http.StatusBadGateway)
			// Log to HAR even if there's an error sending the request (resp might be nil)
			h.proxy.logToHAR(r, nil, startTime, timeTaken, false)
			h.proxy.notifyError(err, reqCtx)
			return
		}
		defer resp.Body.Close()

		if resp.Request == nil {
			resp.Request = outReq
		}

		// Log to HAR - 但对于SSE响应，我们需要特殊处理
		if h.proxy.HarLogger != nil && h.proxy.HarLogger.IsEnabled() {
			serverIP := ""
			if outReq != nil && outReq.URL != nil {
				serverIP = outReq.URL.Host
			}

			// 检查是否是SSE响应
			if isServerSentEvent(resp) {
				// 对于SSE响应，创建一个没有响应体的副本，以避免读取整个响应体
				respCopy := *resp
				respCopy.Body = nil
				h.proxy.HarLogger.AddEntry(r, &respCopy, startTime, timeTaken, serverIP, r.RemoteAddr)
			} else {
				// 对于非SSE响应，正常记录
				h.proxy.HarLogger.AddEntry(r, resp, startTime, timeTaken, serverIP, r.RemoteAddr)
			}
		}

		// 处理压缩的响应体
		h.proxy.processCompressedResponse(resp, reqCtx, h.proxy.Verbose)

		// 创建响应上下文
		respCtx := h.proxy.createResponseContext(reqCtx, resp, timeTaken)

		// 通知响应事件
		modifiedResp := h.proxy.notifyResponse(respCtx)
		if modifiedResp != resp && modifiedResp != nil {
			resp = modifiedResp
			respCtx.Response = resp
		}

		// Log to HAR - 但对于SSE响应，我们在 handleSSE 中记录
		// 只为非 SSE 响应记录 HAR 条目
		if !isServerSentEvent(resp) {
			h.proxy.logToHAR(r, resp, startTime, timeTaken, false)
		}

		// Check if this is actually an SSE response
		if isServerSentEvent(resp) {
			if h.proxy.Verbose {
				log.Printf("[HTTP/2] Confirmed SSE response from %s", targetURL.String())
			}

			// Handle SSE response
			err := h.proxy.handleSSE(w, respCtx)
			if err != nil {
				log.Printf("[SSE] Error handling SSE response: %v", err)
			}
			return
		} else {
			// Not an SSE response, handle normally
			if h.proxy.Verbose {
				log.Printf("[HTTP/2] Expected SSE but got %s from %s", resp.Header.Get("Content-Type"), targetURL.String())
			}

			// Copy headers from target server's response to our response writer
			for k, vv := range resp.Header {
				for _, v := range vv {
					w.Header().Add(k, v)
				}
			}

			// 添加协议版本头以便前端识别
			w.Header().Add("X-Protocol", r.Proto)

			// Set the status code
			w.WriteHeader(resp.StatusCode)

			// Copy the body from target server's response to our response writer
			written, err := io.Copy(w, resp.Body)
			if err != nil {
				log.Printf("[HTTP/2] Error copying response body: %v", err)
				// Don't send http.Error here as headers might have already been written
			}

			if h.proxy.Verbose {
				log.Printf("[HTTP/2] Copied %d bytes for response body from %s", written, targetURL.String())
			}
			return
		}
	}

	// For non-SSE requests, use normal handling
	client := &http.Client{
		Transport: sseTransport,
		Timeout:   30 * time.Second,
	}

	resp, err := client.Do(outReq)
	timeTaken := time.Since(startTime)

	if err != nil {
		log.Printf("[HTTP/2] Error sending request to target server %s: %v", targetURL.String(), err)
		http.Error(w, fmt.Sprintf("Error proxying to %s: %v", targetURL.String(), err), http.StatusBadGateway)
		// Log to HAR even if there's an error sending the request (resp might be nil)
		if h.proxy.HarLogger != nil && h.proxy.HarLogger.IsEnabled() {
			serverIP := ""
			if outReq != nil && outReq.URL != nil {
				serverIP = outReq.URL.Host
			}
			h.proxy.HarLogger.AddEntry(r, nil, startTime, timeTaken, serverIP, r.RemoteAddr)
		}
		h.proxy.notifyError(err, reqCtx)
		return
	}
	defer resp.Body.Close()

	// 处理压缩的响应体
	h.proxy.processCompressedResponse(resp, reqCtx, h.proxy.Verbose)

	// 创建响应上下文
	respCtx := h.proxy.createResponseContext(reqCtx, resp, timeTaken)

	// 通知响应事件
	modifiedResp := h.proxy.notifyResponse(respCtx)
	if modifiedResp != resp && modifiedResp != nil {
		resp = modifiedResp
		respCtx.Response = resp
	}

	if resp.Request == nil {
		resp.Request = outReq
	}

	// Log to HAR - 但对于SSE响应，我们需要特殊处理
	if h.proxy.HarLogger != nil && h.proxy.HarLogger.IsEnabled() {
		serverIP := ""
		if outReq != nil && outReq.URL != nil {
			serverIP = outReq.URL.Host
		}

		// 检查是否是SSE响应
		if isServerSentEvent(resp) {
			// 对于SSE响应，创建一个没有响应体的副本，以避免读取整个响应体
			respCopy := *resp
			respCopy.Body = nil
			h.proxy.HarLogger.AddEntry(r, &respCopy, startTime, timeTaken, serverIP, r.RemoteAddr)
		} else {
			// 对于非SSE响应，正常记录
			h.proxy.HarLogger.AddEntry(r, resp, startTime, timeTaken, serverIP, r.RemoteAddr)
		}
	}

	// 如果启用了流量输出，输出请求和响应内容
	if h.proxy.DumpTraffic {
		h.proxy.dumpRequestBody(r)
		if !isServerSentEvent(resp) { // SSE响应在handleSSE中处理
			h.proxy.dumpResponseBody(resp)
		}
	}

	if h.proxy.Verbose {
		log.Printf("[HTTP/2] Received response from %s: %d %s", targetURL.String(), resp.StatusCode, resp.Status)
	} else {
		log.Printf("[HTTP/2] %s %s%s -> %d %s", r.Method, r.Host, r.URL.RequestURI(), resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	// Check if this is a Server-Sent Events response
	if isServerSentEvent(resp) {
		if h.proxy.Verbose {
			log.Printf("[HTTP/2] Detected Server-Sent Events response from %s", targetURL.String())
		}

		// 对于 SSE 响应，不在这里记录 HAR 条目，而是在 handleSSE 中完成后记录

		// Handle SSE response
		err := h.proxy.handleSSE(w, respCtx)
		if err != nil {
			log.Printf("[SSE] Error handling SSE response: %v", err)
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

	// 添加协议版本头以便前端识别
	w.Header().Add("X-Protocol", "HTTP/2")

	// Set the status code
	w.WriteHeader(resp.StatusCode)

	// Copy the body from target server's response to our response writer
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("[HTTP/2] Error copying response body: %v", err)
		// Don't send http.Error here as headers might have already been written
	}

	if h.proxy.Verbose {
		log.Printf("[HTTP/2] Copied %d bytes for response body from %s", written, targetURL.String())
	}
}
