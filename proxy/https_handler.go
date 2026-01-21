package proxy

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/LubyRuffy/ProxyCraft/certs"
)

var (
	errSSEStreamHandled      = errors.New("sse stream handled")
	errHijackingNotSupported = errors.New("hijacking not supported")
)

// handleHTTPS handles CONNECT requests for MITM or direct tunneling
func (s *Server) handleHTTPS(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received CONNECT request for: %s", r.Host)

	session, err := newHTTPSConnectSession(s, w, r)
	if err != nil {
		if errors.Is(err, errHijackingNotSupported) {
			http.Error(w, "hijacking not supported", http.StatusInternalServerError)
		}
		log.Printf("Failed to establish CONNECT session for %s: %v", r.Host, err)
		return
	}
	defer session.Close()

	session.logNegotiatedProtocol()

	if session.usesHTTP2() {
		session.proxyHTTP2()
		return
	}

	if err := session.proxyHTTP1(); err != nil {
		log.Printf("[MITM for %s] Error handling tunneled requests: %v", r.Host, err)
	}
}

type httpsConnectSession struct {
	server          *Server
	connectReq      *http.Request
	hostPort        string
	hostname        string
	rawConn         net.Conn
	tlsConn         *tls.Conn
	negotiatedProto string
}

func newHTTPSConnectSession(server *Server, w http.ResponseWriter, r *http.Request) (*httpsConnectSession, error) {
	hostPort := ensurePort(r.Host)
	server.notifyTunnelEstablished(hostPort, true)

	hijacker, ok := w.(http.Hijacker)
	if !ok {
		return nil, errHijackingNotSupported
	}

	rawConn, rw, err := hijacker.Hijack()
	if err != nil {
		return nil, fmt.Errorf("error hijacking connection: %w", err)
	}

	if err := sendConnectionEstablished(r, rw); err != nil {
		_ = rawConn.Close()
		return nil, err
	}

	hostname := extractHostname(r.Host)

	tlsConn, negotiatedProto, err := server.startMITMTLS(rawConn, hostname, r.RemoteAddr)
	if err != nil {
		_ = rawConn.Close()
		return nil, err
	}

	return &httpsConnectSession{
		server:          server,
		connectReq:      r,
		hostPort:        hostPort,
		hostname:        hostname,
		rawConn:         rawConn,
		tlsConn:         tlsConn,
		negotiatedProto: negotiatedProto,
	}, nil
}

func (s *httpsConnectSession) Close() {
	if s.tlsConn != nil {
		_ = s.tlsConn.Close()
		s.tlsConn = nil
	} else if s.rawConn != nil {
		_ = s.rawConn.Close()
		s.rawConn = nil
	}
}

func (s *httpsConnectSession) logNegotiatedProtocol() {
	if !s.server.Verbose {
		return
	}
	proto := s.negotiatedProto
	if proto == "" {
		proto = "http/1.1"
	}
	log.Printf("[MITM for %s] Negotiated protocol: %s", s.connectReq.Host, proto)
}

func (s *httpsConnectSession) usesHTTP2() bool {
	return s.negotiatedProto == "h2"
}

func (s *httpsConnectSession) proxyHTTP2() {
	s.server.handleHTTP2MITM(s.tlsConn, s.connectReq)
}

func (s *httpsConnectSession) proxyHTTP1() error {
	defer func() {
		if s.server.Verbose {
			log.Printf("[MITM for %s] Exiting MITM processing loop.", s.connectReq.Host)
		}
	}()

	clientReader := bufio.NewReader(s.tlsConn)
	for {
		tunneledReq, err := http.ReadRequest(clientReader)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				log.Printf("[MITM for %s] Client closed connection or EOF: %v", s.connectReq.Host, err)
				return nil
			}
			if opError, ok := err.(*net.OpError); ok && opError.Err != nil && opError.Err.Error() == "tls: use of closed connection" {
				log.Printf("[MITM for %s] TLS connection closed by client: %v", s.connectReq.Host, err)
				return nil
			}
			log.Printf("[MITM for %s] Error reading request from client: %v", s.connectReq.Host, err)
			return fmt.Errorf("read tunneled request: %w", err)
		}

		log.Printf("[MITM for %s] Received tunneled request: %s %s%s %s",
			s.connectReq.Host,
			tunneledReq.Method,
			tunneledReq.Host,
			tunneledReq.URL.String(),
			tunneledReq.Proto,
		)

		if err := s.handleTunneledRequest(tunneledReq); err != nil {
			if errors.Is(err, errSSEStreamHandled) {
				return nil
			}
			return err
		}
	}
}

func (s *httpsConnectSession) handleTunneledRequest(tunneledReq *http.Request) error {
	targetURL := &url.URL{
		Scheme:   "https",
		Host:     s.connectReq.Host,
		Path:     tunneledReq.URL.Path,
		RawQuery: tunneledReq.URL.RawQuery,
	}

	baseTransport := s.server.newTransport(s.connectReq.Host, true)
	transport := s.server.wrapTransportForSSE(baseTransport)

	proxyReq, reqCtx, potentialSSE, startTime, err := s.server.prepareProxyRequest(tunneledReq, targetURL.String(), true)
	if err != nil {
		writeGatewayError(s.tlsConn, s.connectReq.Proto)
		return fmt.Errorf("create proxy request: %w", err)
	}

	logPotentialSSE(s.server.Verbose, "[Proxy]", potentialSSE)

	resp, timeTaken, err := s.server.sendProxyRequest(proxyReq, transport, potentialSSE, startTime)
	if err != nil {
		s.server.recordProxyError(err, reqCtx, startTime, timeTaken)
		writeGatewayError(s.tlsConn, s.connectReq.Proto)
		return fmt.Errorf("send proxy request: %w", err)
	}
	defer resp.Body.Close()

	respCtx, isSSE := s.server.processProxyResponse(reqCtx, resp, startTime, timeTaken, "[Proxy]", targetURL.String())

	if isSSE {
		if err := s.server.streamSSEOverTLS(s.tlsConn, respCtx, s.connectReq.Proto); err != nil {
			s.server.notifyError(err, reqCtx)
			return fmt.Errorf("stream SSE over TLS: %w", err)
		}
		return errSSEStreamHandled
	}

	if err := s.server.tunnelHTTPSResponse(s.tlsConn, respCtx.Response, reqCtx); err != nil {
		return fmt.Errorf("tunnel HTTPS response: %w", err)
	}

	return nil
}

func sendConnectionEstablished(r *http.Request, rw *bufio.ReadWriter) error {
	if rw == nil {
		return fmt.Errorf("connection writer unavailable")
	}
	if _, err := rw.WriteString(r.Proto + " 200 Connection Established\r\n\r\n"); err != nil {
		return fmt.Errorf("error writing 200 response: %w", err)
	}
	if err := rw.Flush(); err != nil {
		return fmt.Errorf("error flushing response: %w", err)
	}
	return nil
}

func (s *Server) startMITMTLS(conn net.Conn, hostname, clientAddr string) (*tls.Conn, string, error) {
	tlsConfig, err := s.tlsConfigForHost(hostname)
	if err != nil {
		return nil, "", err
	}

	tlsConn := tls.Server(conn, tlsConfig)
	if err := tlsConn.Handshake(); err != nil {
		log.Printf("TLS handshake error with client %s for host %s: %v", clientAddr, hostname, err)
		if strings.Contains(err.Error(), "bad certificate") {
			log.Printf("TLS MITM hint: ensure the ProxyCraft Root CA in system trust store matches %s", certs.MustGetCACertPath())
			log.Printf("TLS MITM hint: restart the client after updating trust; some apps (e.g. Firefox) use their own trust store")
		}
		return nil, "", err
	}

	log.Printf("Successfully completed TLS handshake with client for %s", hostname)

	state := tlsConn.ConnectionState()
	return tlsConn, state.NegotiatedProtocol, nil
}

func (s *Server) tlsConfigForHost(hostname string) (*tls.Config, error) {
	log.Printf("Generating certificate for hostname: %s", hostname)
	serverCert, serverKey, err := s.CertManager.GenerateServerCert(hostname)
	if err != nil {
		log.Printf("Error generating server certificate for %s: %v", hostname, err)
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{
			{
				Certificate: [][]byte{serverCert.Raw},
				PrivateKey:  serverKey,
			},
		},
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
		NextProtos: []string{"h2", "http/1.1"},
	}, nil
}

func ensurePort(host string) string {
	if strings.Contains(host, ":") {
		return host
	}
	return host + ":443"
}

func extractHostname(host string) string {
	if name, _, err := net.SplitHostPort(host); err == nil {
		return name
	}
	return host
}

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
		// 使用通用的流式传输函数处理响应
		contentType := resp.Header.Get("Content-Type")
		_, err := s.streamResponse(resp.Body, clientConn, contentType, s.Verbose)
		if err != nil {
			return fmt.Errorf("流式传输响应出错: %w", err)
		}
	}

	return nil
}

func (s *Server) streamSSEOverTLS(conn *tls.Conn, respCtx *ResponseContext, clientProto string) error {
	if conn == nil || respCtx == nil || respCtx.Response == nil {
		return fmt.Errorf("invalid SSE context")
	}

	if s.Verbose {
		target := ""
		if respCtx.Response.Request != nil && respCtx.Response.Request.URL != nil {
			target = respCtx.Response.Request.URL.String()
		}
		log.Printf("[Proxy] Detected Server-Sent Events response from %s", target)
	}

	writer := newTLSResponseWriter(conn, clientProto)
	if err := s.handleSSE(writer, respCtx); err != nil {
		return err
	}
	return nil
}

func writeGatewayError(conn net.Conn, proto string) {
	if conn == nil {
		return
	}
	if proto == "" {
		proto = "HTTP/1.1"
	}
	_, _ = conn.Write([]byte(fmt.Sprintf("%s 502 Bad Gateway\r\n\r\n", proto)))
}

type tlsResponseWriter struct {
	conn        *tls.Conn
	buf         *bufio.Writer
	header      http.Header
	status      int
	proto       string
	wroteHeader bool
}

func newTLSResponseWriter(conn *tls.Conn, proto string) *tlsResponseWriter {
	return &tlsResponseWriter{
		conn:   conn,
		buf:    bufio.NewWriter(conn),
		header: make(http.Header),
		proto:  proto,
	}
}

func (w *tlsResponseWriter) Header() http.Header {
	return w.header
}

func (w *tlsResponseWriter) Write(data []byte) (int, error) {
	if !w.wroteHeader {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.buf.Write(data)
	if err != nil {
		return n, err
	}
	if err := w.buf.Flush(); err != nil {
		return n, err
	}
	return n, nil
}

func (w *tlsResponseWriter) WriteHeader(status int) {
	if w.wroteHeader {
		return
	}
	if w.proto == "" {
		w.proto = "HTTP/1.1"
	}
	statusText := http.StatusText(status)
	if statusText == "" {
		statusText = "Status"
	}
	_, _ = w.buf.WriteString(fmt.Sprintf("%s %d %s\r\n", w.proto, status, statusText))
	for k, vv := range w.header {
		for _, v := range vv {
			_, _ = w.buf.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
		}
	}
	_, _ = w.buf.WriteString("\r\n")
	_ = w.buf.Flush()
	w.status = status
	w.wroteHeader = true
}

func (w *tlsResponseWriter) Flush() {
	_ = w.buf.Flush()
}

var _ interface {
	http.ResponseWriter
	http.Flusher
} = (*tlsResponseWriter)(nil)
