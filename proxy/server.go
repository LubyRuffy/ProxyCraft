package proxy

import (
	"bufio" // Added for reading requests from TLS connection
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
	Addr        string
	CertManager *certs.Manager
	Verbose     bool
	HarLogger   *harlogger.Logger // Added for HAR logging
	EnableMITM  bool              // 是否启用MITM模式，默认为false表示直接隧道模式
}

// NewServer creates a new proxy server instance
func NewServer(addr string, certManager *certs.Manager, verbose bool, harLogger *harlogger.Logger, enableMITM bool) *Server {
	return &Server{
		Addr:        addr,
		CertManager: certManager,
		Verbose:     verbose,
		HarLogger:   harLogger,
		EnableMITM:  enableMITM,
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

	// Create a transport with HTTP/2 support
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Configure HTTP/2 support
	s.handleHTTP2(transport)

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
		if s.HarLogger.IsEnabled() {
			serverIP := ""
			if proxyReq != nil && proxyReq.URL != nil {
				serverIP = proxyReq.URL.Host
			}
			s.HarLogger.AddEntry(r, nil, startTime, timeTaken, serverIP, r.RemoteAddr)
		}
		return
	}
	defer resp.Body.Close()

	// Log to HAR
	if s.HarLogger.IsEnabled() {
		serverIP := ""
		if proxyReq != nil && proxyReq.URL != nil {
			serverIP = proxyReq.URL.Host
		}
		s.HarLogger.AddEntry(r, resp, startTime, timeTaken, serverIP, r.RemoteAddr)
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
		// 连接到目标服务器
		targetConn, err := net.DialTimeout("tcp", hostPort, 10*time.Second)
		if err != nil {
			log.Printf("Error connecting to target server %s: %v", hostPort, err)
			http.Error(w, fmt.Sprintf("无法连接到目标服务器: %v", err), http.StatusBadGateway)
			return
		}
		defer targetConn.Close()

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

		// 发送200 OK响应
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
		log.Printf("Establishing direct tunnel to %s", hostPort)
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
		}

		// Configure HTTP/2 support for the transport
		s.handleHTTP2(transport)

		httpClient := &http.Client{
			Transport: transport,
			Timeout:   30 * time.Second,
		}
		resp, err := httpClient.Do(outReq)
		timeTaken := time.Since(startTime)

		if err != nil {
			log.Printf("[MITM for %s] Error sending request to target %s: %v", r.Host, targetURL.String(), err)
			// Log to HAR even if there's an error
			if s.HarLogger.IsEnabled() {
				connectionID := ""
				if tlsClientConn != nil {
					connectionID = tlsClientConn.RemoteAddr().String()
				}
				s.HarLogger.AddEntry(tunneledReq, nil, startTime, timeTaken, r.Host, connectionID)
			}
			break
		}

		// Log to HAR
		if s.HarLogger.IsEnabled() {
			connectionID := ""
			if tlsClientConn != nil {
				connectionID = tlsClientConn.RemoteAddr().String()
			}
			s.HarLogger.AddEntry(tunneledReq, resp, startTime, timeTaken, r.Host, connectionID)
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
				if s.Verbose && len(line) > 1 { // Skip empty lines
					lineStr := strings.TrimSpace(string(line))
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
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "text/event-stream")
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
	}

	// Configure HTTP/2 support for the transport
	h.proxy.handleHTTP2(transport)

	client := &http.Client{
		Transport: transport,
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

	// Log to HAR
	if h.proxy.HarLogger.IsEnabled() {
		serverIP := ""
		if outReq != nil && outReq.URL != nil {
			serverIP = outReq.URL.Host
		}
		h.proxy.HarLogger.AddEntry(r, resp, startTime, timeTaken, serverIP, r.RemoteAddr)
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

// handleSSE handles Server-Sent Events responses
func (s *Server) handleSSE(w http.ResponseWriter, resp *http.Response) error {
	// Set appropriate headers for SSE
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// Set the status code
	w.WriteHeader(resp.StatusCode)

	// Create a flusher if the ResponseWriter supports it
	flusher, ok := w.(http.Flusher)
	if !ok {
		return fmt.Errorf("streaming not supported")
	}

	// Log SSE handling
	if s.Verbose {
		log.Printf("[SSE] Handling Server-Sent Events stream")
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

		// Write the event data to the client
		_, err = w.Write(line)
		if err != nil {
			return fmt.Errorf("error writing SSE data to client: %v", err)
		}

		// Log the event if verbose
		if s.Verbose && len(line) > 1 { // Skip empty lines
			lineStr := strings.TrimSpace(string(line))
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

		// Flush the data to the client immediately
		flusher.Flush()
	}

	return nil
}
