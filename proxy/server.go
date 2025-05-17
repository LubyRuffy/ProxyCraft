package proxy

import (
	"bufio" // Added for reading requests from TLS connection
	"bytes" // Added for bytes.Buffer
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url" // Added for constructing target URLs
	"strings"
	"time"

	"github.com/LubyRuffy/ProxyCraft/certs"
	"github.com/LubyRuffy/ProxyCraft/harlogger" // Added for HAR logging
	"golang.org/x/net/http2"                    // Added for HTTP/2 support
)

// Server struct will hold proxy server configuration and state
type Server struct {
	Addr          string
	CertManager   *certs.Manager
	Verbose       bool
	HarLogger     *harlogger.Logger // Added for HAR logging
	EnableMITM    bool              // 是否启用MITM模式，默认为false表示直接隧道模式
	UpstreamProxy *url.URL          // 上层代理服务器URL，如果为nil则直接连接
}

// NewServer creates a new proxy server instance
func NewServer(addr string, certManager *certs.Manager, verbose bool, harLogger *harlogger.Logger, enableMITM bool, upstreamProxy *url.URL) *Server {
	return &Server{
		Addr:          addr,
		CertManager:   certManager,
		Verbose:       verbose,
		HarLogger:     harLogger,
		EnableMITM:    enableMITM,
		UpstreamProxy: upstreamProxy,
	}
}

// Start begins listening for incoming proxy requests
func (s *Server) Start() error {
	fmt.Printf("Proxy server starting on %s\n", s.Addr)
	//http.HandleFunc("/", s.handleHTTP)
	return http.ListenAndServe(s.Addr, http.HandlerFunc(s.handleHTTP))
}

// handleHTTP is the handler for all incoming HTTP requests
func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	if s.Verbose {
		log.Printf("[HTTP] Received request: %s %s %s %s", r.Method, r.Host, r.URL.String(), r.Proto)
		logHeader(r.Header, "[HTTP] Request Headers:")
	} else {
		log.Printf("[HTTP] %s %s%s", r.Method, r.Host, r.URL.RequestURI())
	}

	if r.Method == http.MethodConnect {
		s.handleHTTPS(w, r)
		return
	}

	// Create a new request to the target server
	// We need to change r.RequestURI to r.URL.String() because r.RequestURI is only set by the server
	// and r.URL is what we want to use for client requests.
	// Also, r.URL.Host is empty for proxy requests, so we use r.Host.
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

	proxyReq, err := http.NewRequest(r.Method, targetURL, r.Body)
	if err != nil {
		log.Printf("[HTTP] Error creating proxy request for %s: %v", targetURL, err)
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
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

	// Send the request to the target server
	startTime := time.Now()

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
			return
		}
		defer resp.Body.Close()

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
		return
	}
	defer resp.Body.Close()

	// Log to HAR - 但对于SSE响应，我们在 handleSSE 中记录
	// 只为非 SSE 响应记录 HAR 条目
	if !isServerSentEvent(resp) {
		s.logToHAR(r, resp, startTime, timeTaken, false)
	}

	if s.Verbose {
		log.Printf("[HTTP] Received response from %s: %d %s", targetURL, resp.StatusCode, resp.Status)
		logHeader(resp.Header, fmt.Sprintf("[HTTP] Response Headers from %s:", targetURL))
	} else {
		log.Printf("[HTTP] %s %s%s -> %d %s", r.Method, r.Host, r.URL.RequestURI(), resp.StatusCode, resp.Header.Get("Content-Type"))
	}

	// Check if this is a Server-Sent Events response
	if isServerSentEvent(resp) {
		if s.Verbose {
			log.Printf("[HTTP] Detected Server-Sent Events response from %s", targetURL)
		}

		// 对于 SSE 响应，不在这里记录 HAR 条目，而是在 handleSSE 中完成后记录
		// 删除之前添加的 HAR 条目（如果有的话）
		// 注意：这里没有实际删除功能，因为 Logger 没有提供删除条目的方法

		// Handle SSE response
		err := s.handleSSE(w, resp)
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

	// Set the status code
	w.WriteHeader(resp.StatusCode)

	// Copy the body from target server's response to our response writer
	written, err := io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("Error copying response body: %v", err)
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

// handleHTTPS handles CONNECT requests for MITM or direct tunneling
func (s *Server) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received CONNECT request for: %s\n", r.Host)

	// 解析目标主机和端口
	hostPort := r.Host
	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":443" // 默认HTTPS端口
	}

	// 直接隧道模式 - 不使用MITM
	if !s.EnableMITM {
		// 劫持客户端连接
		hijacker, ok := w.(http.Hijacker)
		if !ok {
			log.Println("Hijacking not supported")
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
			return
		}
		clientConn, clientWriter, err := hijacker.Hijack()
		if err != nil {
			log.Printf("Error hijacking connection: %v", err)
			http.Error(w, "error hijacking connection", http.StatusInternalServerError)
			return
		}
		defer clientConn.Close()

		var targetConn net.Conn

		// 检查是否使用上层代理
		if s.UpstreamProxy != nil && s.UpstreamProxy.Scheme != "" {
			// 通过上层代理连接
			if s.Verbose {
				log.Printf("[HTTPS] Using upstream proxy %s for connection to %s", s.UpstreamProxy.String(), hostPort)
			}

			// 连接到上层代理
			proxyConn, err := net.DialTimeout("tcp", s.UpstreamProxy.Host, 10*time.Second)
			if err != nil {
				log.Printf("Error connecting to upstream proxy %s: %v", s.UpstreamProxy.Host, err)
				http.Error(w, fmt.Sprintf("无法连接到上层代理服务器: %v", err), http.StatusBadGateway)
				return
			}

			// 发送CONNECT请求到上层代理
			connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", hostPort, hostPort)
			if _, err := proxyConn.Write([]byte(connectReq)); err != nil {
				log.Printf("Error sending CONNECT request to upstream proxy: %v", err)
				proxyConn.Close()
				http.Error(w, fmt.Sprintf("无法发送CONNECT请求到上层代理: %v", err), http.StatusBadGateway)
				return
			}

			// 读取上层代理的响应
			bufReader := bufio.NewReader(proxyConn)
			resp, err := http.ReadResponse(bufReader, &http.Request{Method: "CONNECT"})
			if err != nil {
				log.Printf("Error reading response from upstream proxy: %v", err)
				proxyConn.Close()
				http.Error(w, fmt.Sprintf("无法读取上层代理响应: %v", err), http.StatusBadGateway)
				return
			}

			// 检查上层代理的响应状态
			if resp.StatusCode != http.StatusOK {
				log.Printf("Upstream proxy responded with non-200 status: %d %s", resp.StatusCode, resp.Status)
				proxyConn.Close()
				http.Error(w, fmt.Sprintf("上层代理返回错误状态: %d %s", resp.StatusCode, resp.Status), http.StatusBadGateway)
				return
			}

			// 使用上层代理连接作为目标连接
			targetConn = proxyConn
			if s.Verbose {
				log.Printf("[HTTPS] Successfully established connection to %s via upstream proxy", hostPort)
			}
		} else {
			// 直接连接到目标服务器
			targetConn, err = net.DialTimeout("tcp", hostPort, 10*time.Second)
			if err != nil {
				log.Printf("Error connecting to target server %s: %v", hostPort, err)
				http.Error(w, fmt.Sprintf("无法连接到目标服务器: %v", err), http.StatusBadGateway)
				return
			}
		}
		defer targetConn.Close()

		// 发送200 OK响应给客户端
		responseStr := "HTTP/1.1 200 Connection Established\r\n\r\n"
		if _, err := clientWriter.WriteString(responseStr); err != nil {
			log.Printf("Error writing 200 response: %v", err)
			return
		}
		if err := clientWriter.Flush(); err != nil {
			log.Printf("Error flushing response: %v", err)
			return
		}

		// 创建双向数据转发
		log.Printf("Establishing tunnel to %s", hostPort)
		go func() {
			_, _ = io.Copy(targetConn, clientConn)
		}()
		_, _ = io.Copy(clientConn, targetConn)
		return
	}

	// 以下是MITM模式的处理逻辑
	// 劫持客户端连接
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		log.Println("Hijacking not supported")
		http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		return
	}
	clientConn, clientWriter, err := hijacker.Hijack()
	if err != nil {
		log.Printf("Error hijacking connection: %v", err)
		http.Error(w, "error hijacking connection", http.StatusInternalServerError)
		return
	}
	defer clientConn.Close()

	// Send 200 Connection Established response
	responseStr := "HTTP/1.1 200 Connection Established\r\n\r\n"
	if _, err := clientWriter.WriteString(responseStr); err != nil {
		log.Printf("Error writing 200 response: %v", err)
		return
	}
	if err := clientWriter.Flush(); err != nil {
		log.Printf("Error flushing response: %v", err)
		return
	}

	// 以下是MITM模式的处理逻辑
	// 生成服务器证书
	// Extract hostname without port for certificate generation
	hostname := r.Host
	if h, _, err := net.SplitHostPort(r.Host); err == nil {
		hostname = h
	}
	log.Printf("Generating certificate for hostname: %s", hostname)

	serverCert, serverKey, err := s.CertManager.GenerateServerCert(hostname)
	if err != nil {
		log.Printf("Error generating server certificate for %s: %v", hostname, err)
		return
	}

	// Start TLS with the client using the generated certificate
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{serverCert.Raw},
				PrivateKey:  serverKey,
			},
		},
		MinVersion: tls.VersionTLS12, // Minimum TLS version
		MaxVersion: tls.VersionTLS13, // Maximum TLS version
		// Use modern cipher suites
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		// Enable HTTP/2 ALPN negotiation
		NextProtos: []string{"h2", "http/1.1"},
	}

	tlsClientConn := tls.Server(clientConn, tlsConfig)
	err = tlsClientConn.Handshake()
	if err != nil {
		log.Printf("TLS handshake error with client %s for host %s: %v", r.RemoteAddr, r.Host, err)
		_ = clientConn.Close() // Ensure original connection is closed
		return
	}
	defer tlsClientConn.Close()

	log.Printf("Successfully completed TLS handshake with client for %s", r.Host)

	// Check if the client negotiated HTTP/2
	connState := tlsClientConn.ConnectionState()
	protocol := connState.NegotiatedProtocol

	if s.Verbose {
		log.Printf("[MITM for %s] Negotiated protocol: %s", r.Host, protocol)
	}

	// Handle HTTP/2 connections differently
	if protocol == "h2" {
		s.handleHTTP2MITM(tlsClientConn, r)
		return
	}

	// For HTTP/1.1, handle requests coming over this TLS connection (tlsClientConn)
	clientReader := bufio.NewReader(tlsClientConn)
	for {
		// Read the request from the client over the TLS tunnel
		tunneledReq, err := http.ReadRequest(clientReader)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				log.Printf("[MITM for %s] Client closed connection or EOF: %v", r.Host, err)
			} else if opError, ok := err.(*net.OpError); ok && opError.Err.Error() == "tls: use of closed connection" {
				log.Printf("[MITM for %s] TLS connection closed by client: %v", r.Host, err)
			} else {
				log.Printf("[MITM for %s] Error reading request from client: %v", r.Host, err)
			}
			break // Exit loop on error or EOF
		}

		if s.Verbose {
			log.Printf("[MITM for %s] Received tunneled request: %s %s%s %s", r.Host, tunneledReq.Method, tunneledReq.Host, tunneledReq.URL.String(), tunneledReq.Proto)
			logHeader(tunneledReq.Header, fmt.Sprintf("[MITM for %s] Tunneled Request Headers:", r.Host))
		} else {
			log.Printf("[MITM for %s] %s %s%s", r.Host, tunneledReq.Method, tunneledReq.Host, tunneledReq.URL.RequestURI())
		}

		// Prepare the outgoing request to the actual target server
		// Use the original host from the CONNECT request (with port if present)
		targetHost := r.Host

		// If the tunneled request has a different host, log it (unusual case)
		if tunneledReq.Host != "" && tunneledReq.Host != targetHost && tunneledReq.Host != hostname {
			log.Printf("[MITM for %s] Warning: Tunneled request has different host: %s", r.Host, tunneledReq.Host)
		}

		targetURL := &url.URL{
			Scheme:   "https",    // Because it was a CONNECT request, implying HTTPS
			Host:     targetHost, // The actual destination server from CONNECT
			Path:     tunneledReq.URL.Path,
			RawQuery: tunneledReq.URL.RawQuery,
		}

		outReq, err := http.NewRequest(tunneledReq.Method, targetURL.String(), tunneledReq.Body)
		if err != nil {
			log.Printf("[MITM for %s] Error creating outgoing request: %v", r.Host, err)
			break
		}

		// Copy headers from tunneled request to outgoing request
		outReq.Header = make(http.Header)
		for k, vv := range tunneledReq.Header {
			outReq.Header[k] = vv
		}
		// Set the Host header for the outgoing request to the actual target host
		// Use the hostname without port if the original request had a Host header with just the hostname
		if tunneledReq.Host != "" && (tunneledReq.Host == hostname || tunneledReq.Host == targetHost) {
			outReq.Host = tunneledReq.Host
		} else {
			outReq.Host = targetHost // Use the original host from CONNECT request
		}

		// Send the outgoing request
		startTime := time.Now()

		// Check if this might be an SSE request based on patterns and headers
		potentialSSE := isSSERequest(outReq)
		if s.Verbose && potentialSSE {
			log.Printf("[MITM for %s] Potential SSE request detected based on URL path or Accept header", r.Host)
		}

		// Create a custom transport that skips certificate verification
		// This is necessary for MITM mode to work with HTTPS sites
		// Extract hostname without port for SNI
		targetHostname := targetHost
		if h, _, err := net.SplitHostPort(targetHost); err == nil {
			targetHostname = h
		}

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
		if s.UpstreamProxy != nil {
			if s.Verbose {
				log.Printf("[MITM for %s] Using upstream proxy: %s", r.Host, s.UpstreamProxy.String())
			}
			transport.Proxy = http.ProxyURL(s.UpstreamProxy)
		}

		// Configure HTTP/2 support for the transport
		s.handleHTTP2(transport)

		// Create a custom transport that can detect SSE responses early
		sseTransport := &earlySSEDetector{
			base:           transport,
			responseWriter: tlsClientConn,
			server:         s,
			verbose:        s.Verbose,
		}

		// Special handling for potential SSE requests
		if potentialSSE {
			if s.Verbose {
				log.Printf("[MITM for %s] Using special SSE handling", r.Host)
			}

			// Use a custom client with no timeout for SSE
			httpClient := &http.Client{
				Transport: sseTransport,
				// No timeout for SSE requests
			}

			// Set special headers for SSE
			outReq.Header.Set("Accept", "text/event-stream")
			outReq.Header.Set("Cache-Control", "no-cache")
			outReq.Header.Set("Connection", "keep-alive")

			// Send the request
			resp, err := httpClient.Do(outReq)
			timeTaken := time.Since(startTime)

			if err != nil {
				log.Printf("[MITM for %s] Error sending request to target %s: %v", r.Host, targetURL.String(), err)
				// Log to HAR even if there's an error
				s.logToHAR(tunneledReq, nil, startTime, timeTaken, false)
				break
			}
			defer resp.Body.Close()

			// Log to HAR
			s.logToHAR(tunneledReq, resp, startTime, timeTaken, isServerSentEvent(resp))

			// Check if this is actually an SSE response
			if isServerSentEvent(resp) {
				if s.Verbose {
					log.Printf("[MITM for %s] Confirmed SSE response", r.Host)
				}

				// 对于 SSE 响应，不在这里记录 HAR 条目，而是在处理完成后记录

				// For SSE in MITM mode, we need to handle it differently
				// First, write the response headers
				writer := bufio.NewWriter(tlsClientConn)

				// Write the status line
				statusLine := fmt.Sprintf("HTTP/%d.%d %d %s\r\n",
					resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.Status)
				writer.WriteString(statusLine)

				// Write headers
				for k, vv := range resp.Header {
					for _, v := range vv {
						writer.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
					}
				}

				// End of headers
				writer.WriteString("\r\n")
				writer.Flush()

				// 创建一个缓冲区来收集完整的 SSE 数据
				buffer := &bytes.Buffer{}

				// Now read and forward SSE events
				reader := bufio.NewReader(resp.Body)
				for {
					line, err := reader.ReadBytes('\n')
					if err != nil {
						if err == io.EOF {
							break
						}
						log.Printf("[MITM for %s] Error reading SSE stream: %v", r.Host, err)
						break
					}

					// 写入客户端
					_, err = writer.Write(line)
					if err != nil {
						log.Printf("[MITM for %s] Error writing SSE data to client: %v", r.Host, err)
						break
					}

					// 同时写入缓冲区，用于后续的 HAR 记录
					_, bufErr := buffer.Write(line)
					if bufErr != nil {
						log.Printf("[MITM for %s] Error writing SSE data to buffer: %v", r.Host, bufErr)
					}

					// Log the event if verbose
					lineStr := strings.TrimSpace(string(line))
					logSSEEvent(lineStr, s.Verbose)

					// Flush the data to the client immediately
					writer.Flush()
				}

				// 流结束后，记录 HAR 条目
				if s.HarLogger.IsEnabled() {
					// 创建一个新的响应，包含收集到的完整数据
					newResp := &http.Response{
						Status:     resp.Status,
						StatusCode: resp.StatusCode,
						Header:     resp.Header.Clone(),
						Body:       io.NopCloser(bytes.NewReader(buffer.Bytes())),
						Proto:      resp.Proto,
						ProtoMajor: resp.ProtoMajor,
						ProtoMinor: resp.ProtoMinor,
					}

					// 记录 HAR 条目，包含完整的 SSE 数据
					s.logToHAR(tunneledReq, newResp, startTime, timeTaken, false) // 这里使用 false 因为我们已经有了完整的数据

					if s.Verbose {
						log.Printf("[MITM for %s] Recorded complete SSE response in HAR log (%d bytes)", r.Host, buffer.Len())
					}
				}

				resp.Body.Close()
				// After SSE stream ends, we need to break the loop to close the connection
				break
			} else {
				// Not an SSE response, handle normally
				if s.Verbose {
					log.Printf("[MITM for %s] Expected SSE but got %s", r.Host, resp.Header.Get("Content-Type"))
				}

				// For non-SSE responses, proceed with normal handling
				// Write the response back to the client over the TLS tunnel
				err = resp.Write(tlsClientConn)
				if err != nil {
					log.Printf("[MITM for %s] Error writing response to client: %v", r.Host, err)
					resp.Body.Close()
					break
				}
				resp.Body.Close()

				// Handle connection persistence
				if tunneledReq.Close || resp.Close || tunneledReq.Header.Get("Connection") == "close" || resp.Header.Get("Connection") == "close" {
					if s.Verbose {
						log.Printf("[MITM for %s] Connection close signaled in headers or by request/response close flag.", r.Host)
					}
					break
				}
				continue
			}
		}

		// For non-SSE requests, use normal handling
		httpClient := &http.Client{
			Transport: sseTransport,
			Timeout:   30 * time.Second,
		}
		resp, err := httpClient.Do(outReq)
		timeTaken := time.Since(startTime)

		if err != nil {
			log.Printf("[MITM for %s] Error sending request to target %s: %v", r.Host, targetURL.String(), err)
			// Log to HAR even if there's an error
			s.logToHAR(tunneledReq, nil, startTime, timeTaken, false)
			break
		}

		// Log to HAR - 但对于SSE响应，我们在处理 SSE 时记录
		// 只为非 SSE 响应记录 HAR 条目
		if !isServerSentEvent(resp) {
			s.logToHAR(tunneledReq, resp, startTime, timeTaken, false)
		}

		if s.Verbose {
			log.Printf("[MITM for %s] Received response from %s: %d %s", r.Host, targetURL.String(), resp.StatusCode, resp.Status)
			logHeader(resp.Header, fmt.Sprintf("[MITM for %s] Response Headers from %s:", r.Host, targetURL.String()))
		} else {
			log.Printf("[MITM for %s] %s %s%s -> %d %s", r.Host, tunneledReq.Method, tunneledReq.Host, tunneledReq.URL.RequestURI(), resp.StatusCode, resp.Header.Get("Content-Type"))
		}

		// Check if this is a Server-Sent Events response
		if isServerSentEvent(resp) {
			if s.Verbose {
				log.Printf("[MITM for %s] Detected Server-Sent Events response", r.Host)
			}

			// For SSE in MITM mode, we need to handle it differently
			// First, write the response headers
			writer := bufio.NewWriter(tlsClientConn)

			// Write the status line
			statusLine := fmt.Sprintf("HTTP/%d.%d %d %s\r\n",
				resp.ProtoMajor, resp.ProtoMinor, resp.StatusCode, resp.Status)
			writer.WriteString(statusLine)

			// Write headers
			for k, vv := range resp.Header {
				for _, v := range vv {
					writer.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
				}
			}

			// End of headers
			writer.WriteString("\r\n")
			writer.Flush()

			// Now read and forward SSE events
			reader := bufio.NewReader(resp.Body)
			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err == io.EOF {
						break
					}
					log.Printf("[MITM for %s] Error reading SSE stream: %v", r.Host, err)
					break
				}

				// Write the event data to the client
				_, err = writer.Write(line)
				if err != nil {
					log.Printf("[MITM for %s] Error writing SSE data to client: %v", r.Host, err)
					break
				}

				// Log the event if verbose
				lineStr := strings.TrimSpace(string(line))
				logSSEEvent(lineStr, s.Verbose)

				// Flush the data to the client immediately
				writer.Flush()
			}

			resp.Body.Close()
			// After SSE stream ends, we need to break the loop to close the connection
			break
		} else {
			// For non-SSE responses, proceed with normal handling
			// Write the response back to the client over the TLS tunnel
			err = resp.Write(tlsClientConn)
			if err != nil {
				log.Printf("[MITM for %s] Error writing response to client: %v", r.Host, err)
				resp.Body.Close()
				break
			}
			resp.Body.Close()
		}

		// Handle connection persistence
		if tunneledReq.Close || resp.Close || tunneledReq.Header.Get("Connection") == "close" || resp.Header.Get("Connection") == "close" {
			if s.Verbose {
				log.Printf("[MITM for %s] Connection close signaled in headers or by request/response close flag.", r.Host)
			}
			break
		}
	}
	if s.Verbose {
		log.Printf("[MITM for %s] Exiting MITM processing loop.", r.Host)
	}
}

// logHeader prints HTTP headers if verbose mode is enabled.
func logHeader(header http.Header, prefix string) {
	// This function is called within s.Verbose checks, so no need to check s.Verbose again here.
	log.Println(prefix)
	for k, vv := range header {
		for _, v := range vv {
			log.Printf("  %s: %s", k, v)
		}
	}
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

// logToHAR 是一个辅助方法，用于统一处理 HAR 日志记录
// 这个方法集中了所有 HAR 日志记录逻辑，避免代码重复
func (s *Server) logToHAR(req *http.Request, resp *http.Response, startTime time.Time, timeTaken time.Duration, isSSE bool) {
	if !s.HarLogger.IsEnabled() {
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

// handleHTTP2MITM handles HTTP/2 connections in MITM mode
func (s *Server) handleHTTP2MITM(tlsConn *tls.Conn, connectReq *http.Request) {
	if s.Verbose {
		log.Printf("[HTTP/2] Handling HTTP/2 connection for %s", connectReq.Host)
	}

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
		logHeader(r.Header, "[HTTP/2] Request Headers:")
	} else {
		log.Printf("[HTTP/2] %s %s%s", r.Method, r.Host, r.URL.RequestURI())
	}

	// Create a new request to the target server
	targetURL := &url.URL{
		Scheme:   "https",
		Host:     h.originalReq.Host,
		Path:     r.URL.Path,
		RawQuery: r.URL.RawQuery,
	}

	outReq, err := http.NewRequest(r.Method, targetURL.String(), r.Body)
	if err != nil {
		log.Printf("[HTTP/2] Error creating outgoing request: %v", err)
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
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
	startTime := time.Now()

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
			return
		}
		defer resp.Body.Close()

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
			err := h.proxy.handleSSE(w, resp)
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
		if h.proxy.HarLogger.IsEnabled() {
			serverIP := ""
			if outReq != nil && outReq.URL != nil {
				serverIP = outReq.URL.Host
			}
			h.proxy.HarLogger.AddEntry(r, nil, startTime, timeTaken, serverIP, r.RemoteAddr)
		}
		return
	}
	defer resp.Body.Close()

	// Log to HAR - 但对于SSE响应，我们需要特殊处理
	if h.proxy.HarLogger.IsEnabled() {
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

	if h.proxy.Verbose {
		log.Printf("[HTTP/2] Received response from %s: %d %s", targetURL.String(), resp.StatusCode, resp.Status)
		logHeader(resp.Header, fmt.Sprintf("[HTTP/2] Response Headers from %s:", targetURL.String()))
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
		err := h.proxy.handleSSE(w, resp)
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
func (s *Server) handleSSE(w http.ResponseWriter, resp *http.Response) error {
	// 记录开始时间，用于后续的 HAR 记录
	startTime := time.Now()

	// Set appropriate headers for SSE
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// Ensure critical headers are set for SSE streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Del("Content-Length") // Remove Content-Length to ensure chunked encoding

	// Set the status code
	w.WriteHeader(resp.StatusCode)

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

	// Read and forward SSE events
	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading SSE stream: %v", err)
		}

		// 写入 tee，它会同时写入客户端和缓冲区
		_, err = tee.Write(line)
		if err != nil {
			return fmt.Errorf("error writing SSE data: %v", err)
		}

		// Log the event if verbose
		lineStr := strings.TrimSpace(string(line))
		logSSEEvent(lineStr, s.Verbose)
	}

	// 流结束后，记录 HAR 条目
	if s.HarLogger.IsEnabled() {
		// 计算流处理时间
		timeTaken := time.Since(startTime)

		// 创建一个新的响应，包含收集到的完整数据
		newResp := &http.Response{
			Status:     resp.Status,
			StatusCode: resp.StatusCode,
			Header:     resp.Header.Clone(),
			Body:       io.NopCloser(bytes.NewReader(tee.GetBuffer().Bytes())),
			Proto:      resp.Proto,
			ProtoMajor: resp.ProtoMajor,
			ProtoMinor: resp.ProtoMinor,
		}

		// 使用原始请求记录 HAR 条目
		s.logToHAR(resp.Request, newResp, startTime, timeTaken, false) // 这里使用 false 因为我们已经有了完整的数据

		if s.Verbose {
			log.Printf("[SSE] Recorded complete SSE response in HAR log (%d bytes)", tee.GetBuffer().Len())
		}
	}

	return nil
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
