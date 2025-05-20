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

// ServerConfig 包含所有服务器配置项
type ServerConfig struct {
	// 监听地址
	Addr string

	// 证书管理器
	CertManager *certs.Manager

	// 是否输出详细日志
	Verbose bool

	// HAR 日志记录器
	HarLogger *harlogger.Logger

	// 上游代理 URL
	UpstreamProxy *url.URL

	// 是否将抓包内容输出到控制台
	DumpTraffic bool

	// 事件处理器
	EventHandler EventHandler
}

// Server struct will hold proxy server configuration and state
type Server struct {
	Addr          string
	CertManager   *certs.Manager
	Verbose       bool
	HarLogger     *harlogger.Logger // Added for HAR logging
	UpstreamProxy *url.URL          // 上层代理服务器URL，如果为nil则直接连接
	DumpTraffic   bool              // 是否将抓包内容输出到控制台
	EventHandler  EventHandler      // 事件处理器
}

// NewServer creates a new proxy server instance
func NewServer(addr string, certManager *certs.Manager, verbose bool, harLogger *harlogger.Logger, upstreamProxy *url.URL, dumpTraffic bool) *Server {
	return &Server{
		Addr:          addr,
		CertManager:   certManager,
		Verbose:       verbose,
		HarLogger:     harLogger,
		UpstreamProxy: upstreamProxy,
		DumpTraffic:   dumpTraffic,
		EventHandler:  &NoOpEventHandler{}, // 默认使用空实现
	}
}

// NewServerWithConfig 使用配置创建新的代理服务器实例
func NewServerWithConfig(config ServerConfig) *Server {
	server := &Server{
		Addr:          config.Addr,
		CertManager:   config.CertManager,
		Verbose:       config.Verbose,
		HarLogger:     config.HarLogger,
		UpstreamProxy: config.UpstreamProxy,
		DumpTraffic:   config.DumpTraffic,
		EventHandler:  config.EventHandler,
	}

	// 如果没有提供事件处理器，使用默认的空实现
	if server.EventHandler == nil {
		server.EventHandler = &NoOpEventHandler{}
	}

	return server
}

// SetEventHandler 设置事件处理器
func (s *Server) SetEventHandler(handler EventHandler) {
	s.EventHandler = handler
}

// Start begins listening for incoming proxy requests
func (s *Server) Start() error {
	fmt.Printf("Proxy server starting on %s\n", s.Addr)
	return http.ListenAndServe(s.Addr, http.HandlerFunc(s.handleHTTP))
}
