package proxy

import (
	"bufio"
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
		responseStr := r.Proto + " 200 Connection Established\r\n\r\n"
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
	responseStr := r.Proto + " 200 Connection Established\r\n\r\n"
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
		targetHost := r.Host
		targetURL := &url.URL{
			Scheme:   "https",
			Host:     targetHost,
			Path:     tunneledReq.URL.Path,
			RawQuery: tunneledReq.URL.RawQuery,
		}

		// 构建transport
		targetHostname := targetHost
		if h, _, err := net.SplitHostPort(targetHost); err == nil {
			targetHostname = h
		}
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         targetHostname,
			},
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableCompression:    true,
		}
		if s.UpstreamProxy != nil {
			if s.Verbose {
				log.Printf("[MITM for %s] Using upstream proxy: %s", r.Host, s.UpstreamProxy.String())
			}
			transport.Proxy = http.ProxyURL(s.UpstreamProxy)
		}
		s.handleHTTP2(transport)
		// 用统一逻辑处理MITM代理
		s.handleProxyRequest(nil, tunneledReq, targetURL.String(), transport, true, tlsClientConn)
	}
	if s.Verbose {
		log.Printf("[MITM for %s] Exiting MITM processing loop.", r.Host)
	}
}

// tunnelHTTPSResponse 处理HTTPS隧道响应，返回到客户端
func (s *Server) tunnelHTTPSResponse(clientConn *tls.Conn, resp *http.Response, reqCtx *RequestContext) error {
	// 创建一个用于存储响应头的映射
	respHeader := make(http.Header)

	// 复制响应头
	for k, vv := range resp.Header {
		for _, v := range vv {
			respHeader.Add(k, v)
		}
	}

	// 添加协议版本头以便前端识别
	respHeader.Add("X-Protocol", resp.Request.Proto)

	// 处理压缩的响应体
	s.processCompressedResponse(resp, reqCtx, s.Verbose)

	// 写入响应状态行
	statusLine := fmt.Sprintf("%s %s\r\n", resp.Proto, resp.Status)
	if _, err := clientConn.Write([]byte(statusLine)); err != nil {
		return fmt.Errorf("写入状态行到客户端出错: %w", err)
	}

	// 写入响应头
	for k, vv := range respHeader {
		for _, v := range vv {
			headerLine := fmt.Sprintf("%s: %s\r\n", k, v)
			if _, err := clientConn.Write([]byte(headerLine)); err != nil {
				return fmt.Errorf("写入响应头到客户端出错: %w", err)
			}
		}
	}

	// 写入空行，表示头部结束
	if _, err := clientConn.Write([]byte("\r\n")); err != nil {
		return fmt.Errorf("写入头部结束分隔符到客户端出错: %w", err)
	}

	// 写入响应体
	if resp.Body != nil {
		_, err := io.Copy(clientConn, resp.Body)
		if err != nil {
			return fmt.Errorf("写入响应体到客户端出错: %w", err)
		}
	}

	return nil
}
