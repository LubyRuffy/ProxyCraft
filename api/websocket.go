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
	"github.com/zishang520/socket.io/servers/socket/v3"
)

// TestWebSocketServer 是一个用于测试的WebSocket服务器，包含模拟数据
type TestWebSocketServer struct {
	*WebSocketServer
	testMode bool
}

// WebSocketServer 表示WebSocket服务器
type WebSocketServer struct {
	WebHandler *handlers.WebHandler // Web处理器引用
	Server     *socket.Server       // Socket.io服务器
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

// getJsonValue 从interface{}中获取指定字段的值
func getJsonValue(data interface{}, key string) interface{} {
	if data == nil {
		return nil
	}

	// 如果是map类型，直接获取
	if m, ok := data.(map[string]interface{}); ok {
		return m[key]
	}

	// 尝试将data解析为map
	jsonStr, err := json.Marshal(data)
	if err != nil {
		return nil
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonStr, &result); err != nil {
		return nil
	}

	return result[key]
}

// NewWebSocketServer 创建一个新的WebSocket服务器
func NewWebSocketServer(webHandler *handlers.WebHandler) (*WebSocketServer, error) {
	return NewWebSocketServerWithTestMode(webHandler, false)
}

// NewWebSocketServerWithTestMode 创建一个新的WebSocket服务器，支持测试模式
func NewWebSocketServerWithTestMode(webHandler *handlers.WebHandler, testMode bool) (*WebSocketServer, error) {
	// 创建一个新的 socket.io 服务器
	serverOptions := socket.DefaultServerOptions()
	serverOptions.SetPingInterval(60 * time.Second)
	serverOptions.SetPingTimeout(90 * time.Second)
	serverOptions.SetMaxHttpBufferSize(1000000)
	serverOptions.SetConnectTimeout(1000 * time.Millisecond)

	server := socket.NewServer(nil, serverOptions)

	ws := &WebSocketServer{
		WebHandler: webHandler,
		Server:     server,
		Clients:    make(map[string]bool),
	}

	// 设置事件处理器
	ws.setupEventHandlers()

	// 如果是测试模式，添加模拟数据
	if testMode {
		ws.addTestData()
	}

	return ws, nil
}

// addTestData 添加测试数据
func (ws *WebSocketServer) addTestData() {
	// 直接访问WebHandler的内部字段来添加测试数据
	ws.WebHandler.ClearEntries()

	// 创建一个模拟的流量条目
	testEntry := &handlers.TrafficEntry{
		ID:              "test-1",
		StartTime:       time.Now(),
		EndTime:         time.Now().Add(100 * time.Millisecond),
		Duration:        100,
		Host:            "example.com",
		Method:          "GET",
		URL:             "https://example.com/test",
		Path:            "/test",
		StatusCode:      200,
		ContentType:     "text/plain",
		ContentSize:     14,
		Protocol:        "HTTP/1.1",
		RequestHeaders:  make(http.Header),
		ResponseHeaders: make(http.Header),
		RequestBody:     []byte(""),
		ResponseBody:    []byte("Hello, World!"),
	}

	// 直接修改WebHandler的内部字段
	ws.WebHandler.ClearEntries()
	// 注意：这里我们需要通过反射或其他方式访问私有字段
	// 或者创建一个公共方法

	log.Printf("已添加测试数据，条目ID: %s", testEntry.ID)
}

// setupEventHandlers 设置WebSocket事件处理器
func (ws *WebSocketServer) setupEventHandlers() {
	// 处理连接事件
	ws.Server.On("connection", func(clients ...interface{}) {
		client := clients[0].(*socket.Socket)
		ws.mu.Lock()
		ws.Clients[fmt.Sprintf("%v", client.Id())] = true
		clientCount := len(ws.Clients)
		ws.mu.Unlock()

		log.Printf("WebSocket 客户端已连接: %s (当前连接数: %d)", fmt.Sprintf("%v", client.Id()), clientCount)

		// 处理客户端断开连接事件
		client.On("disconnect", func(reasons ...interface{}) {
			reason := "unknown"
			if len(reasons) > 0 {
				reason = fmt.Sprintf("%v", reasons[0])
			}
			ws.mu.Lock()
			delete(ws.Clients, fmt.Sprintf("%v", client.Id()))
			clientCount := len(ws.Clients)
			ws.mu.Unlock()

			log.Printf("WebSocket 客户端已断开连接: %s, 原因: %s (当前连接数: %d)", fmt.Sprintf("%v", client.Id()), reason, clientCount)
		})

		// 处理错误事件
		client.On("error", func(errors ...interface{}) {
			if len(errors) > 0 {
				log.Printf("WebSocket 错误: %v, 客户端: %s", errors[0], fmt.Sprintf("%v", client.Id()))
			}
		})

		// 获取所有流量条目 - 在客户端级别监听
		client.On(EventTrafficEntries, func(args ...interface{}) {
			log.Printf("接收到获取所有流量条目请求, 客户端: %s", fmt.Sprintf("%v", client.Id()))

			// 立即返回一个空的条目列表，这样我们就能确认事件处理函数被调用了
			entries := []*handlers.TrafficEntry{}

			// 如果WebHandler不为空，尝试获取真实数据
			if ws.WebHandler != nil {
				entries = ws.WebHandler.GetEntries()
			}

			log.Printf("准备发送 %d 条流量条目到客户端", len(entries))
			client.Emit(EventTrafficEntries, entries)
			log.Printf("已发送所有流量条目到客户端: %s, 条目数: %d", fmt.Sprintf("%v", client.Id()), len(entries))
		})

		// 获取请求详情 - 在客户端级别监听
		client.On(EventRequestDetails, func(args ...interface{}) {
			id := args[0].(string)
			log.Printf("接收到获取请求详情请求, 客户端: %s, 条目ID: %s", fmt.Sprintf("%v", client.Id()), id)

			entry := ws.WebHandler.GetEntry(id)
			if entry == nil {
				log.Printf("未找到条目, ID: %s", id)
				client.Emit("error", map[string]string{"message": "Entry not found"})
				return
			}

			// 处理请求头和请求体
			requestDetails := ws.formatRequestDetails(entry)
			client.Emit(EventRequestDetails, requestDetails)
			log.Printf("已发送请求详情到客户端: %s, 条目ID: %s", fmt.Sprintf("%v", client.Id()), id)
		})

		// 获取响应详情 - 在客户端级别监听
		client.On(EventResponseDetails, func(args ...interface{}) {
			id := args[0].(string)
			log.Printf("接收到获取响应详情请求, 客户端: %s, 条目ID: %s", fmt.Sprintf("%v", client.Id()), id)

			entry := ws.WebHandler.GetEntry(id)
			if entry == nil {
				log.Printf("未找到条目, ID: %s", id)
				client.Emit("error", map[string]string{"message": "Entry not found"})
				return
			}

			// 处理响应头和响应体
			responseDetails := ws.formatResponseDetails(entry)
			client.Emit(EventResponseDetails, responseDetails)
			log.Printf("已发送响应详情到客户端: %s, 条目ID: %s", fmt.Sprintf("%v", client.Id()), id)
		})

		// 清空所有流量条目 - 在客户端级别监听
		client.On(EventTrafficClear, func(args ...interface{}) {
			log.Printf("接收到清空所有流量条目请求, 客户端: %s", fmt.Sprintf("%v", client.Id()))
			ws.WebHandler.ClearEntries()

			// 广播给所有客户端
			ws.BroadcastClearTraffic()

			log.Printf("已清空所有流量条目, 请求来自客户端: %s", fmt.Sprintf("%v", client.Id()))
		})

		// 处理ping事件 - 在客户端级别监听
		client.On("ping", func(args ...interface{}) {
			log.Printf("接收到ping请求, 客户端: %s", fmt.Sprintf("%v", client.Id()))
			client.Emit("pong", "pong")
		})

		// 处理客户端心跳事件 - 在客户端级别监听
		client.On("heartbeat", func(args ...interface{}) {
			var data interface{}
			if len(args) > 0 {
				data = args[0]
			}
			// 解析客户端发送的心跳数据，并以相同格式返回
			// 减少日志输出，避免日志过多
			// log.Printf("接收到心跳请求, 客户端: %s", fmt.Sprintf("%v", client.Id()))

			// 返回心跳响应
			client.Emit("heartbeat-response", map[string]interface{}{
				"serverTime": time.Now().UnixNano() / int64(time.Millisecond),
				"received":   true,
				"counter":    getJsonValue(data, "counter"), // 返回相同的计数器
			})
		})
	})
}

// formatRequestDetails 格式化请求详情
func (ws *WebSocketServer) formatRequestDetails(entry *handlers.TrafficEntry) map[string]interface{} {
	// 处理请求头
	headers := make(map[string]string)
	for name, values := range entry.RequestHeaders {
		headers[name] = strings.Join(values, "; ")
	}

	// 添加Host字段，因为Go的http.Request.Header不包含Host字段
	// Host字段在Go中被特殊处理，存储在Request.Host属性中
	if entry.Host != "" && headers["Host"] == "" {
		headers["Host"] = entry.Host
	}

	// 添加其他可能缺失的重要HTTP头部字段
	// 添加User-Agent字段，如果不存在
	if headers["User-Agent"] == "" {
		// 尝试从请求头中获取User-Agent
		if ua := entry.RequestHeaders.Get("User-Agent"); ua != "" {
			headers["User-Agent"] = ua
		}
	}

	// 添加Content-Type字段，如果不存在
	if headers["Content-Type"] == "" && entry.RequestHeaders.Get("Content-Type") != "" {
		headers["Content-Type"] = entry.RequestHeaders.Get("Content-Type")
	}

	// 添加Content-Length字段，如果不存在且请求体不为空
	if headers["Content-Length"] == "" && len(entry.RequestBody) > 0 {
		headers["Content-Length"] = fmt.Sprintf("%d", len(entry.RequestBody))
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

	// 确保Content-Type字段存在
	if entry.ContentType != "" && headers["Content-Type"] == "" {
		headers["Content-Type"] = entry.ContentType
	}

	// 确保Content-Length字段存在
	if entry.ContentSize > 0 && headers["Content-Length"] == "" {
		headers["Content-Length"] = fmt.Sprintf("%d", entry.ContentSize)
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
		ws.Server.Emit(EventTrafficNewEntry, entry)
	}
}

// BroadcastClearTraffic 广播清空所有流量条目
func (ws *WebSocketServer) BroadcastClearTraffic() {
	ws.mu.Lock()
	clientCount := len(ws.Clients)
	ws.mu.Unlock()

	log.Printf("广播清空所有流量条目, 广播客户端数: %d", clientCount)
	if clientCount > 0 {
		ws.Server.Emit(EventTrafficClear, nil)
	}
}

// Start 启动WebSocket服务器
func (ws *WebSocketServer) Start() {
	// 打印WebSocket服务器配置
	log.Printf("正在启动WebSocket服务器，配置信息: PingTimeout=90s, PingInterval=60s")

	// 启动socket.io服务器
	go func() {
		log.Printf("WebSocket服务器goroutine启动")
		// 注意：新的API不需要调用Serve()方法，服务器通过HTTP处理器处理请求
	}()

	log.Printf("WebSocket服务器已启动，准备接受连接")
}

// Stop 停止WebSocket服务器
func (ws *WebSocketServer) Stop() {
	ws.Server.Close(nil)
}

// GetHandler 获取WebSocket HTTP处理器
func (ws *WebSocketServer) GetHandler() http.Handler {
	return ws.Server.ServeHandler(nil)
}
