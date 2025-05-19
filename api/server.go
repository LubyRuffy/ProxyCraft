package api

import (
	"embed"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/LubyRuffy/ProxyCraft/proxy/handlers"
	"github.com/gin-gonic/gin"
)

//go:embed dist
var dist embed.FS

// Server 表示API服务器
type Server struct {
	WebHandler      *handlers.WebHandler // Web处理器引用
	Router          *gin.Engine          // Gin路由
	UIPort          int                  // UI服务端口
	UIAddr          string               // UI服务地址
	StaticDir       string               // 静态文件目录
	Dist            embed.FS             // 嵌入的静态文件
	WebSocketServer *WebSocketServer     // WebSocket服务器
}

// NewServer 创建一个新的API服务器
func NewServer(webHandler *handlers.WebHandler, port int) *Server {
	// 设置Gin为发布模式
	gin.SetMode(gin.ReleaseMode)

	// 创建服务器
	server := &Server{
		WebHandler: webHandler,
		Router:     gin.Default(),
		UIPort:     port,
		UIAddr:     fmt.Sprintf("http://localhost:%d", port),
		// StaticDir:  "./api/dist", // 默认静态文件目录
		Dist: dist,
	}

	// 确保静态文件目录存在
	if server.StaticDir != "" {
		if _, err := os.Stat(server.StaticDir); os.IsNotExist(err) {
			log.Printf("Warning: Static directory %s does not exist", server.StaticDir)
		}
	}

	// 初始化WebSocket服务器
	wsServer, err := NewWebSocketServer(webHandler)
	if err != nil {
		log.Printf("Warning: Could not initialize WebSocket server: %v", err)
	} else {
		server.WebSocketServer = wsServer
	}

	// 配置路由
	server.setupRoutes()

	return server
}

// setupRoutes 设置API路由
func (s *Server) setupRoutes() {
	// API路由组
	api := s.Router.Group("/api")
	{
		// 获取所有流量条目
		api.GET("/traffic", s.getTrafficEntries)

		// 获取特定流量条目的详细信息
		api.GET("/traffic/:id", s.getTrafficEntry)

		// 清空所有流量条目
		api.DELETE("/traffic", s.clearTrafficEntries)

		// 获取请求头和请求体
		api.GET("/traffic/:id/request", s.getRequestDetails)

		// 获取响应头和响应体
		api.GET("/traffic/:id/response", s.getResponseDetails)
	}

	// WebSocket服务路由
	if s.WebSocketServer != nil {
		s.Router.GET("/socket.io/*any", gin.WrapH(s.WebSocketServer.GetHandler()))
		s.Router.POST("/socket.io/*any", gin.WrapH(s.WebSocketServer.GetHandler()))
	}

	if s.StaticDir != "" {
		// 前端静态文件处理
		s.Router.NoRoute(s.serveUI)
	} else {
		// 提供嵌入的静态文件
		// 修改这里，使用精确路由而不是通配符路由，避免与API路由冲突

		// 主页
		s.Router.GET("/", func(c *gin.Context) {
			c.Header("Cache-Control", "no-cache")
			data, _ := s.Dist.ReadFile("dist/index.html")
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		})

		// favicon.ico
		s.Router.GET("/favicon.ico", func(c *gin.Context) {
			data, err := s.Dist.ReadFile("dist/favicon.ico")
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			c.Data(http.StatusOK, "image/x-icon", data)
		})

		// 静态资源文件
		s.Router.GET("/assets/:filename", func(c *gin.Context) {
			filename := c.Param("filename")
			filePath := "dist/assets/" + filename

			data, err := s.Dist.ReadFile(filePath)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}

			// 根据文件扩展名设置Content-Type
			contentType := "application/octet-stream"
			if strings.HasSuffix(filename, ".css") {
				contentType = "text/css"
			} else if strings.HasSuffix(filename, ".js") {
				contentType = "application/javascript"
			} else if strings.HasSuffix(filename, ".png") {
				contentType = "image/png"
			} else if strings.HasSuffix(filename, ".jpg") || strings.HasSuffix(filename, ".jpeg") {
				contentType = "image/jpeg"
			} else if strings.HasSuffix(filename, ".svg") {
				contentType = "image/svg+xml"
			}

			c.Data(http.StatusOK, contentType, data)
		})

		// 处理前端路由和其他静态文件
		s.Router.NoRoute(func(c *gin.Context) {
			// 如果不是API请求，则返回index.html以支持前端路由
			if !strings.HasPrefix(c.Request.URL.Path, "/api") {
				// 防止循环重定向
				c.Header("Cache-Control", "no-cache")
				c.Status(http.StatusOK) // 确保不返回301/302等重定向状态码
				data, _ := s.Dist.ReadFile("dist/index.html")
				c.Data(http.StatusOK, "text/html; charset=utf-8", data)
			} else {
				// API路径但找不到路由，返回404
				c.JSON(http.StatusNotFound, gin.H{"error": "API endpoint not found"})
			}
		})
	}
}

// Start 启动API服务器
func (s *Server) Start() error {
	// 启动WebSocket服务器
	if s.WebSocketServer != nil {
		s.WebSocketServer.Start()
		log.Printf("WebSocket服务已启动")

		// 设置WebHandler的通知回调
		s.WebHandler.SetNewEntryCallback(func(entry *handlers.TrafficEntry) {
			if s.WebSocketServer != nil {
				s.WebSocketServer.BroadcastNewEntry(entry)
			}
		})
	}

	log.Printf("Web UI available at %s", s.UIAddr)
	return s.Router.Run(fmt.Sprintf(":%d", s.UIPort))
}

// getTrafficEntries 返回所有流量条目
func (s *Server) getTrafficEntries(c *gin.Context) {
	entries := s.WebHandler.GetEntries()
	c.JSON(http.StatusOK, gin.H{
		"entries": entries,
	})
}

// getTrafficEntry 返回特定流量条目
func (s *Server) getTrafficEntry(c *gin.Context) {
	id := c.Param("id")
	entry := s.WebHandler.GetEntry(id)

	if entry == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Entry not found",
		})
		return
	}

	c.JSON(http.StatusOK, entry)
}

// clearTrafficEntries 清空所有流量条目
func (s *Server) clearTrafficEntries(c *gin.Context) {
	s.WebHandler.ClearEntries()
	c.JSON(http.StatusOK, gin.H{
		"message": "All traffic entries cleared",
	})
}

// getRequestDetails 获取请求详情
func (s *Server) getRequestDetails(c *gin.Context) {
	id := c.Param("id")
	entry := s.WebHandler.GetEntry(id)

	if entry == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Entry not found",
		})
		return
	}

	// 处理请求头
	headers := make(map[string]string)
	for name, values := range entry.RequestHeaders {
		headers[name] = strings.Join(values, "; ")
	}

	// 处理请求体
	var body interface{}

	contentType := entry.RequestHeaders.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// 尝试解析JSON
		if err := json.Unmarshal(entry.RequestBody, &body); err != nil {
			body = string(entry.RequestBody)
		}
	} else if strings.Contains(contentType, "text/") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "application/javascript") {
		// 文本内容
		body = string(entry.RequestBody)
	} else {
		// 二进制内容
		body = fmt.Sprintf("<Binary data, %d bytes>", len(entry.RequestBody))
	}

	c.JSON(http.StatusOK, gin.H{
		"headers": headers,
		"body":    body,
	})
}

// getResponseDetails 获取响应详情
func (s *Server) getResponseDetails(c *gin.Context) {
	id := c.Param("id")
	entry := s.WebHandler.GetEntry(id)

	if entry == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Entry not found",
		})
		return
	}

	// 处理响应头
	headers := make(map[string]string)
	for name, values := range entry.ResponseHeaders {
		headers[name] = strings.Join(values, "; ")
	}

	// 处理响应体
	var body interface{}

	contentType := entry.ResponseHeaders.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		// 尝试解析JSON
		if err := json.Unmarshal(entry.ResponseBody, &body); err != nil {
			body = string(entry.ResponseBody)
		}
	} else if strings.Contains(contentType, "text/") ||
		strings.Contains(contentType, "application/xml") ||
		strings.Contains(contentType, "application/javascript") {
		// 文本内容
		body = string(entry.ResponseBody)
	} else {
		// 二进制内容
		body = fmt.Sprintf("<Binary data, %d bytes>", len(entry.ResponseBody))
	}

	c.JSON(http.StatusOK, gin.H{
		"headers": headers,
		"body":    body,
	})
}

// serveUI 提供前端静态文件
func (s *Server) serveUI(c *gin.Context) {
	// 如果请求的路径是API路径，则跳过
	if strings.HasPrefix(c.Request.URL.Path, "/api") {
		c.Next()
		return
	}

	// 尝试从静态目录提供文件
	filePath := filepath.Join(s.StaticDir, c.Request.URL.Path)

	// 检查文件是否存在
	stat, err := os.Stat(filePath)
	if err == nil && !stat.IsDir() {
		// 文件存在，提供文件
		c.File(filePath)
		return
	}

	// 如果文件不存在，则提供index.html
	indexPath := filepath.Join(s.StaticDir, "index.html")
	if _, err := os.Stat(indexPath); err == nil {
		// index.html存在，提供之
		c.Header("Cache-Control", "no-cache")
		c.File(indexPath)
		return
	}

	// 静态目录中没有index.html，返回404
	c.JSON(http.StatusNotFound, gin.H{
		"error": "Static file not found",
	})
}
