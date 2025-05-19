package proxy

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

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
		// 通知隧道建立事件
		s.notifyTunnelEstablished(hostPort, false)

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
	// 通知隧道建立事件
	s.notifyTunnelEstablished(hostPort, true)

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

		// 创建请求上下文并通知请求事件
		reqCtx := s.createRequestContext(tunneledReq, targetURL.String(), startTime, true)
		modifiedReq := s.notifyRequest(reqCtx)
		if modifiedReq != tunneledReq && modifiedReq != nil {
			// 如果请求被修改，更新外发请求
			tunneledReq = modifiedReq
			// 重新创建外发请求以包含修改
			outReq, err = http.NewRequest(tunneledReq.Method, targetURL.String(), tunneledReq.Body)
			if err != nil {
				log.Printf("[MITM for %s] Error creating modified outgoing request: %v", r.Host, err)
				break
			}
			// 复制修改后的请求头
			outReq.Header = make(http.Header)
			for k, vv := range tunneledReq.Header {
				outReq.Header[k] = vv
			}
			outReq.Host = tunneledReq.Host
		}

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
				s.notifyError(err, reqCtx)
				break
			}
			defer resp.Body.Close()

			// 创建响应上下文
			respCtx := s.createResponseContext(reqCtx, resp, timeTaken)

			// 通知响应事件
			modifiedResp := s.notifyResponse(respCtx)
			if modifiedResp != resp && modifiedResp != nil {
				resp = modifiedResp
				respCtx.Response = resp
			}

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
			// Log to HAR even if there's an error sending the request (resp might be nil)
			s.logToHAR(tunneledReq, nil, startTime, timeTaken, false)
			s.notifyError(err, reqCtx)
			break
		}

		// 创建响应上下文
		respCtx := s.createResponseContext(reqCtx, resp, timeTaken)

		// 通知响应事件
		modifiedResp := s.notifyResponse(respCtx)
		if modifiedResp != resp && modifiedResp != nil {
			resp = modifiedResp
			respCtx.Response = resp
		}

		// Log to HAR - 但对于SSE响应，我们在处理 SSE 时记录
		// 只为非 SSE 响应记录 HAR 条目
		if !isServerSentEvent(resp) {
			s.logToHAR(tunneledReq, resp, startTime, timeTaken, false)
		}

		// 如果启用了流量输出，输出请求和响应内容
		if s.DumpTraffic {
			s.dumpRequestBody(tunneledReq)
			if !isServerSentEvent(resp) { // SSE响应在处理SSE时输出
				s.dumpResponseBody(resp)
			}
		}

		if s.Verbose {
			log.Printf("[MITM for %s] Received response from %s: %d %s", r.Host, targetURL.String(), resp.StatusCode, resp.Status)
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
