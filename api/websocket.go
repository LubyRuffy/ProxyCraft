package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"

	"github.com/LubyRuffy/ProxyCraft/proxy/handlers"
	socketio "github.com/googollee/go-socket.io"
	"github.com/googollee/go-socket.io/engineio"
	"github.com/googollee/go-socket.io/engineio/transport"
	"github.com/googollee/go-socket.io/engineio/transport/polling"
	"github.com/googollee/go-socket.io/engineio/transport/websocket"
)

// WebSocketServer 表示WebSocket服务器
type WebSocketServer struct {
	WebHandler *handlers.WebHandler // Web处理器引用
	Server     *socketio.Server     // Socket.io服务器
	Clients    map[string]bool      // 连接的客户端
	mu         sync.Mutex           // 互斥锁，用于保护clients
}

// 事件类型常量
const (
	EventConnect         = "connect"           // 连接事件
	EventDisconnect      = "disconnect"        // 断开连接事件
	EventError           = "error"             // 错误事件
	EventTrafficEntries  = "traffic_entries"   // 获取所有流量条目
	EventTrafficNewEntry = "traffic_new_entry" // 新的流量条目
	EventTrafficClear    = "traffic_clear"     // 清空所有流量条目
	EventRequestDetails  = "request_details"   // 请求详情
	EventResponseDetails = "response_details"  // 响应详情
)

// NewWebSocketServer 创建一个新的WebSocket服务器
func NewWebSocketServer(webHandler *handlers.WebHandler) (*WebSocketServer, error) {
	// 创建一个新的 socket.io 服务器
	server := socketio.NewServer(&engineio.Options{
		Transports: []transport.Transport{
			&polling.Transport{
				CheckOrigin: func(r *http.Request) bool {
					return true // 允许所有来源的请求，生产环境中应当限制
				},
			},
			&websocket.Transport{
				CheckOrigin: func(r *http.Request) bool {
					return true // 允许所有来源的请求，生产环境中应当限制
				},
			},
		},
		// 在此设置其他选项，例如超时等
	})

	ws := &WebSocketServer{
		WebHandler: webHandler,
		Server:     server,
		Clients:    make(map[string]bool),
	}

	// 设置事件处理器
	ws.setupEventHandlers()

	return ws, nil
}

// setupEventHandlers 设置WebSocket事件处理器
func (ws *WebSocketServer) setupEventHandlers() {
	// 处理连接事件
	ws.Server.OnConnect("/", func(s socketio.Conn) error {
		ws.mu.Lock()
		ws.Clients[s.ID()] = true
		ws.mu.Unlock()

		log.Printf("WebSocket 客户端已连接: %s", s.ID())
		return nil
	})

	// 处理断开连接事件
	ws.Server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		ws.mu.Lock()
		delete(ws.Clients, s.ID())
		ws.mu.Unlock()

		log.Printf("WebSocket 客户端已断开连接: %s, 原因: %s", s.ID(), reason)
	})

	// 处理错误事件
	ws.Server.OnError("/", func(s socketio.Conn, e error) {
		log.Printf("WebSocket 错误: %s, 客户端: %s", e.Error(), s.ID())
	})

	// 获取所有流量条目
	ws.Server.OnEvent("/", EventTrafficEntries, func(s socketio.Conn) {
		entries := ws.WebHandler.GetEntries()
		s.Emit(EventTrafficEntries, entries)
		log.Printf("已发送所有流量条目到客户端: %s", s.ID())
	})

	// 获取请求详情
	ws.Server.OnEvent("/", EventRequestDetails, func(s socketio.Conn, id string) {
		entry := ws.WebHandler.GetEntry(id)
		if entry == nil {
			s.Emit("error", map[string]string{"message": "Entry not found"})
			return
		}

		// 处理请求头和请求体
		requestDetails := ws.formatRequestDetails(entry)
		s.Emit(EventRequestDetails, requestDetails)
		log.Printf("已发送请求详情到客户端: %s, 条目ID: %s", s.ID(), id)
	})

	// 获取响应详情
	ws.Server.OnEvent("/", EventResponseDetails, func(s socketio.Conn, id string) {
		entry := ws.WebHandler.GetEntry(id)
		if entry == nil {
			s.Emit("error", map[string]string{"message": "Entry not found"})
			return
		}

		// 处理响应头和响应体
		responseDetails := ws.formatResponseDetails(entry)
		s.Emit(EventResponseDetails, responseDetails)
		log.Printf("已发送响应详情到客户端: %s, 条目ID: %s", s.ID(), id)
	})

	// 清空所有流量条目
	ws.Server.OnEvent("/", EventTrafficClear, func(s socketio.Conn) {
		ws.WebHandler.ClearEntries()

		// 广播给所有客户端
		ws.BroadcastClearTraffic()

		log.Printf("已清空所有流量条目, 请求来自客户端: %s", s.ID())
	})
}

// formatRequestDetails 格式化请求详情
func (ws *WebSocketServer) formatRequestDetails(entry *handlers.TrafficEntry) map[string]interface{} {
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

	return map[string]interface{}{
		"headers": headers,
		"body":    body,
	}
}

// formatResponseDetails 格式化响应详情
func (ws *WebSocketServer) formatResponseDetails(entry *handlers.TrafficEntry) map[string]interface{} {
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

	return map[string]interface{}{
		"headers": headers,
		"body":    body,
	}
}

// BroadcastNewEntry 广播新的流量条目给所有客户端
func (ws *WebSocketServer) BroadcastNewEntry(entry *handlers.TrafficEntry) {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	log.Printf("广播新的流量条目, ID: %s", entry.ID)
	ws.Server.BroadcastToNamespace("/", EventTrafficNewEntry, entry)
}

// BroadcastClearTraffic 广播清空所有流量条目
func (ws *WebSocketServer) BroadcastClearTraffic() {
	ws.mu.Lock()
	defer ws.mu.Unlock()

	log.Printf("广播清空所有流量条目")
	ws.Server.BroadcastToNamespace("/", EventTrafficClear, nil)
}

// Start 启动WebSocket服务器
func (ws *WebSocketServer) Start() {
	go func() {
		if err := ws.Server.Serve(); err != nil {
			log.Fatalf("无法启动WebSocket服务器: %v", err)
		}
	}()
}

// Stop 停止WebSocket服务器
func (ws *WebSocketServer) Stop() {
	if err := ws.Server.Close(); err != nil {
		log.Printf("关闭WebSocket服务器时出错: %v", err)
	}
}

// GetHandler 获取WebSocket HTTP处理器
func (ws *WebSocketServer) GetHandler() http.Handler {
	return ws.Server
}
