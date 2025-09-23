package proxy

import (
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"time"
)

// newTransport creates a transport configured for HTTP or HTTPS requests.
func (s *Server) newTransport(targetHost string, secure bool) *http.Transport {
	transport := &http.Transport{
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

	if secure {
		hostForSNI := targetHost
		if host, _, err := net.SplitHostPort(targetHost); err == nil {
			hostForSNI = host
		}
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         hostForSNI,
		}
	}

	if s.UpstreamProxy != nil {
		if s.Verbose {
			log.Printf("[Proxy] Using upstream proxy: %s", s.UpstreamProxy.String())
		}
		transport.Proxy = http.ProxyURL(s.UpstreamProxy)
	}

	s.handleHTTP2(transport)

	return transport
}

// wrapTransportForSSE wraps the base transport so that SSE responses can be detected early.
func (s *Server) wrapTransportForSSE(base *http.Transport) http.RoundTripper {
	if base == nil {
		return nil
	}
	return &earlySSEDetector{
		base:    base,
		server:  s,
		verbose: s.Verbose,
	}
}

// prepareProxyRequest builds the outgoing request and related context for proxying.
func (s *Server) prepareProxyRequest(r *http.Request, targetURL string, isHTTPS bool) (*http.Request, *RequestContext, bool, time.Time, error) {
	startTime := time.Now()

	reqCtx := s.createRequestContext(r, targetURL, startTime, isHTTPS)
	if modified := s.notifyRequest(reqCtx); modified != nil && modified != r {
		r = modified
		reqCtx.Request = modified
	}

	proxyReq, err := cloneRequestWithURL(r, targetURL)
	if err != nil {
		s.notifyError(err, reqCtx)
		return nil, reqCtx, false, startTime, err
	}

	potentialSSE := isSSERequest(proxyReq)

	return proxyReq, reqCtx, potentialSSE, startTime, nil
}

// sendProxyRequest executes the outbound request using the provided transport.
func (s *Server) sendProxyRequest(proxyReq *http.Request, transport http.RoundTripper, potentialSSE bool, startTime time.Time) (*http.Response, time.Duration, error) {
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	if potentialSSE {
		client.Timeout = 0
		proxyReq.Header.Set("Accept", "text/event-stream")
		proxyReq.Header.Set("Cache-Control", "no-cache")
		proxyReq.Header.Set("Connection", "keep-alive")
	}

	resp, err := client.Do(proxyReq)
	timeTaken := time.Since(startTime)
	if err != nil {
		return nil, timeTaken, err
	}

	if resp.Request == nil {
		resp.Request = proxyReq
	}

	return resp, timeTaken, nil
}

// processProxyResponse handles response post-processing, logging, and notifications.
func (s *Server) processProxyResponse(reqCtx *RequestContext, resp *http.Response, startTime time.Time, timeTaken time.Duration, logPrefix, targetURL string) (*ResponseContext, bool) {
	if resp == nil {
		return nil, false
	}

	s.processCompressedResponse(resp, reqCtx, s.Verbose)

	respCtx := s.createResponseContext(reqCtx, resp, timeTaken)
	if modified := s.notifyResponse(respCtx); modified != nil && modified != resp {
		resp = modified
		respCtx.Response = resp
	}

	isSSE := isServerSentEvent(respCtx.Response)

	if s.DumpTraffic {
		s.dumpRequestBody(reqCtx.Request)
		if !isSSE {
			s.dumpResponseBody(respCtx.Response)
		}
	}

	if !isSSE {
		s.logToHAR(reqCtx.Request, respCtx.Response, startTime, timeTaken, false)
	}

	if s.Verbose {
		log.Printf("%s Received response from %s: %d %s", logPrefix, targetURL, respCtx.Response.StatusCode, respCtx.Response.Status)
	} else {
		reqURL := reqCtx.Request.URL
		host := reqCtx.Request.Host
		path := ""
		if reqURL != nil {
			path = reqURL.RequestURI()
		}
		log.Printf("%s %s %s%s -> %d %s", logPrefix, reqCtx.Request.Method, host, path, respCtx.Response.StatusCode, respCtx.Response.Header.Get("Content-Type"))
	}

	return respCtx, isSSE
}

// recordProxyError captures error details for logging and event notification.
func (s *Server) recordProxyError(err error, reqCtx *RequestContext, startTime time.Time, timeTaken time.Duration) {
	if reqCtx == nil {
		return
	}
	s.logToHAR(reqCtx.Request, nil, startTime, timeTaken, false)
	s.notifyError(err, reqCtx)
}

// cloneRequestWithURL duplicates the request and rewrites its URL to the target.
func cloneRequestWithURL(r *http.Request, targetURL string) (*http.Request, error) {
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		return nil, err
	}

	proxyReq.Header = r.Header.Clone()
	proxyReq.Host = r.Host
	proxyReq.ContentLength = r.ContentLength
	if len(r.TransferEncoding) > 0 {
		proxyReq.TransferEncoding = append([]string(nil), r.TransferEncoding...)
	}
	proxyReq.Close = r.Close

	return proxyReq, nil
}

// writeHTTPResponse writes the proxied response back to an HTTP client.
func (s *Server) writeHTTPResponse(w http.ResponseWriter, respCtx *ResponseContext, protocol string) error {
	if respCtx == nil || respCtx.Response == nil {
		return nil
	}

	for k, vv := range respCtx.Response.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	w.Header().Add("X-Protocol", protocol)
	w.WriteHeader(respCtx.Response.StatusCode)

	contentType := respCtx.Response.Header.Get("Content-Type")
	_, err := s.streamResponse(respCtx.Response.Body, w, contentType, s.Verbose)
	return err
}

func logPotentialSSE(verbose bool, prefix string, potential bool) {
	if !verbose || !potential {
		return
	}
	log.Printf("%s Potential SSE request detected based on URL path or Accept header", prefix)
}
