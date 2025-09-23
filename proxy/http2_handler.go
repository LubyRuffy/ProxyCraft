package proxy

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/url"

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

// handleHTTP2MITM handles HTTP/2 connections
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

// http2MITMConn is a connection wrapper for HTTP/2
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

	baseTransport := h.proxy.newTransport(h.originalReq.Host, true)
	transport := h.proxy.wrapTransportForSSE(baseTransport)

	proxyReq, reqCtx, potentialSSE, startTime, err := h.proxy.prepareProxyRequest(r, targetURL.String(), true)
	if err != nil {
		log.Printf("[HTTP/2] Error creating proxy request: %v", err)
		http.Error(w, "Error creating proxy request", http.StatusInternalServerError)
		return
	}

	logPotentialSSE(h.proxy.Verbose, "[HTTP/2]", potentialSSE)

	resp, timeTaken, err := h.proxy.sendProxyRequest(proxyReq, transport, potentialSSE, startTime)
	if err != nil {
		log.Printf("[HTTP/2] Error sending request to target server %s: %v", targetURL.String(), err)
		h.proxy.recordProxyError(err, reqCtx, startTime, timeTaken)
		http.Error(w, fmt.Sprintf("Error proxying to %s: %v", targetURL.String(), err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respCtx, isSSE := h.proxy.processProxyResponse(reqCtx, resp, startTime, timeTaken, "[HTTP/2]", targetURL.String())

	if isSSE {
		if err := h.proxy.handleSSE(w, respCtx); err != nil {
			log.Printf("[SSE] Error handling SSE response: %v", err)
		}
		return
	}

	if err := h.proxy.writeHTTPResponse(w, respCtx, "HTTP/2"); err != nil {
		log.Printf("[HTTP/2] Error streaming response: %v", err)
	}
}
