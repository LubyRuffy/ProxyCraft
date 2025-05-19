package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/LubyRuffy/ProxyCraft/proxy/handlers"
	socketio "github.com/googollee/go-socket.io"
	"github.com/googollee/go-socket.io/engineio"
	"github.com/googollee/go-socket.io/engineio/transport"
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
			&websocket.Transport{
				CheckOrigin: func(r *http.Request) bool {
					log.Printf("WebSocket websocket 来源检查: %s", r.Header.Get("Origin"))
					return true // 允许所有来源的请求，生产环境中应当限制
				},
			},
		},
		// 调整超时时间和心跳间隔，解决频繁断开问题
		PingTimeout:  30 * time.Second, // 增加到30秒
		PingInterval: 40 * time.Second, // 增加到40秒，必须大于PingTimeout
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
		clientCount := len(ws.Clients)
		ws.mu.Unlock()

		log.Printf("WebSocket 客户端已连接: %s (当前连接数: %d)", s.ID(), clientCount)

		// 异步发送当前所有流量条目，避免阻塞连接处理
		go func() {
			// 给客户端连接一点时间
			time.Sleep(100 * time.Millisecond)

			// 创建一个带超时的通道来获取条目
			entriesChan := make(chan []*handlers.TrafficEntry, 1)

			// 在新的goroutine中获取条目
			go func() {
				entries := ws.WebHandler.GetEntries()
				entriesChan <- entries
			}()

			// 设置5秒超时
			select {
			case entries := <-entriesChan:
				// 成功获取条目
				s.Emit(EventTrafficEntries, entries)
				log.Printf("连接时发送所有流量条目到客户端: %s, 条目数: %d", s.ID(), len(entries))
			case <-time.After(5 * time.Second):
				// 超时处理
				log.Printf("连接时获取流量条目超时, 客户端: %s", s.ID())
				s.Emit("error", map[string]string{"message": "获取流量条目超时，请重试或刷新页面"})
			}
		}()

		return nil
	})

	// 处理断开连接事件
	ws.Server.OnDisconnect("/", func(s socketio.Conn, reason string) {
		ws.mu.Lock()
		delete(ws.Clients, s.ID())
		clientCount := len(ws.Clients)
		ws.mu.Unlock()

		log.Printf("WebSocket 客户端已断开连接: %s, 原因: %s (当前连接数: %d)", s.ID(), reason, clientCount)
	})

	// 处理错误事件
	ws.Server.OnError("/", func(s socketio.Conn, e error) {
		log.Printf("WebSocket 错误: %s, 客户端: %s", e.Error(), s.ID())
	})

	// 获取所有流量条目
	ws.Server.OnEvent("/", EventTrafficEntries, func(s socketio.Conn) {
		log.Printf("接收到获取所有流量条目请求, 客户端: %s", s.ID())

		// 创建一个带超时的通道来获取条目
		entriesChan := make(chan []*handlers.TrafficEntry, 1)

		// 在新的goroutine中获取条目，避免阻塞
		go func() {
			entries := ws.WebHandler.GetEntries()
			entriesChan <- entries
		}()

		// 设置5秒超时
		select {
		case entries := <-entriesChan:
			// 成功获取条目
			s.Emit(EventTrafficEntries, entries)
			log.Printf("已发送所有流量条目到客户端: %s, 条目数: %d", s.ID(), len(entries))
		case <-time.After(5 * time.Second):
			// 超时处理
			log.Printf("获取流量条目超时, 客户端: %s", s.ID())
			s.Emit("error", map[string]string{"message": "获取流量条目超时，请重试"})
		}
	})

	// 获取请求详情
	ws.Server.OnEvent("/", EventRequestDetails, func(s socketio.Conn, id string) {
		log.Printf("接收到获取请求详情请求, 客户端: %s, 条目ID: %s", s.ID(), id)
		entry := ws.WebHandler.GetEntry(id)
		if entry == nil {
			log.Printf("未找到条目, ID: %s", id)
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
		log.Printf("接收到获取响应详情请求, 客户端: %s, 条目ID: %s", s.ID(), id)
		entry := ws.WebHandler.GetEntry(id)
		if entry == nil {
			log.Printf("未找到条目, ID: %s", id)
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
		log.Printf("接收到清空所有流量条目请求, 客户端: %s", s.ID())
		ws.WebHandler.ClearEntries()

		// 广播给所有客户端
		ws.BroadcastClearTraffic()

		log.Printf("已清空所有流量条目, 请求来自客户端: %s", s.ID())
	})

	// 处理ping事件
	ws.Server.OnEvent("/", "ping", func(s socketio.Conn) {
		log.Printf("接收到ping请求, 客户端: %s", s.ID())
		s.Emit("pong", "pong")
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
	clientCount := len(ws.Clients)
	ws.mu.Unlock()

	log.Printf("广播新的流量条目, ID: %s, 广播客户端数: %d", entry.ID, clientCount)
	if clientCount > 0 {
		ws.Server.BroadcastToNamespace("/", EventTrafficNewEntry, entry)
	}
}

// BroadcastClearTraffic 广播清空所有流量条目
func (ws *WebSocketServer) BroadcastClearTraffic() {
	ws.mu.Lock()
	clientCount := len(ws.Clients)
	ws.mu.Unlock()

	log.Printf("广播清空所有流量条目, 广播客户端数: %d", clientCount)
	if clientCount > 0 {
		ws.Server.BroadcastToNamespace("/", EventTrafficClear, nil)
	}
}

// Start 启动WebSocket服务器
func (ws *WebSocketServer) Start() {
	// 打印WebSocket服务器配置
	log.Printf("正在启动WebSocket服务器，配置信息: PingTimeout=30s, PingInterval=40s")

	// 启动server.io服务器
	go func() {
		log.Printf("WebSocket服务器goroutine启动")
		if err := ws.Server.Serve(); err != nil {
			log.Printf("WebSocket服务器错误: %v", err)
		}
	}()

	log.Printf("WebSocket服务器已启动，准备接受连接")
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
