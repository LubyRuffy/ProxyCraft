package proxy

import (
	// Added for reading requests from TLS connection
	// Added for bytes.Buffer

	"fmt"
	"net/http"
	"net/url" // Added for constructing target URLs

	"github.com/LubyRuffy/ProxyCraft/certs"
	"github.com/LubyRuffy/ProxyCraft/harlogger" // Added for HAR logging
	// Added for HTTP/2 support
)

// Server struct will hold proxy server configuration and state
type Server struct {
	Addr          string
	CertManager   *certs.Manager
	Verbose       bool
	HarLogger     *harlogger.Logger // Added for HAR logging
	EnableMITM    bool              // 是否启用MITM模式，默认为false表示直接隧道模式
	UpstreamProxy *url.URL          // 上层代理服务器URL，如果为nil则直接连接
	DumpTraffic   bool              // 是否将抓包内容输出到控制台
}

// NewServer creates a new proxy server instance
func NewServer(addr string, certManager *certs.Manager, verbose bool, harLogger *harlogger.Logger, enableMITM bool, upstreamProxy *url.URL, dumpTraffic bool) *Server {
	return &Server{
		Addr:          addr,
		CertManager:   certManager,
		Verbose:       verbose,
		HarLogger:     harLogger,
		EnableMITM:    enableMITM,
		UpstreamProxy: upstreamProxy,
		DumpTraffic:   dumpTraffic,
	}
}

// Start begins listening for incoming proxy requests
func (s *Server) Start() error {
	fmt.Printf("Proxy server starting on %s\n", s.Addr)
	return http.ListenAndServe(s.Addr, http.HandlerFunc(s.handleHTTP))
}
