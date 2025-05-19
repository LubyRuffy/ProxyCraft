package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/LubyRuffy/ProxyCraft/proxy"
)

// TrafficEntry 表示一条流量记录
type TrafficEntry struct {
	ID              string      `json:"id"`              // 唯一标识
	StartTime       time.Time   `json:"startTime"`       // 请求开始时间
	EndTime         time.Time   `json:"endTime"`         // 响应结束时间
	Duration        int64       `json:"duration"`        // 耗时（毫秒）
	Host            string      `json:"host"`            // 主机名
	Method          string      `json:"method"`          // 请求方法
	URL             string      `json:"url"`             // URL
	Path            string      `json:"path"`            // 路径
	StatusCode      int         `json:"statusCode"`      // 状态码
	ContentType     string      `json:"contentType"`     // 内容类型
	ContentSize     int         `json:"contentSize"`     // 内容大小
	IsSSE           bool        `json:"isSSE"`           // 是否为SSE请求
	IsHTTPS         bool        `json:"isHTTPS"`         // 是否为HTTPS请求
	RequestBody     []byte      `json:"-"`               // 请求体
	ResponseBody    []byte      `json:"-"`               // 响应体
	RequestHeaders  http.Header `json:"-"`               // 请求头
	ResponseHeaders http.Header `json:"-"`               // 响应头
	Error           string      `json:"error,omitempty"` // 错误信息
}

// NewEntryCallback 定义新条目回调函数类型
type NewEntryCallback func(entry *TrafficEntry)

// WebHandler 实现了EventHandler接口，用于Web模式
type WebHandler struct {
	entries          []*TrafficEntry          // 流量条目
	entriesMap       map[string]*TrafficEntry // 通过ID查询条目
	entryMutex       sync.RWMutex             // 保护entries和entriesMap的互斥锁
	verbose          bool                     // 是否输出详细日志
	nextID           int                      // 下一个ID
	idMutex          sync.Mutex               // 保护nextID的互斥锁
	newEntryCallback NewEntryCallback         // 新条目回调函数
	callbackMutex    sync.RWMutex             // 保护回调函数的互斥锁
}

// NewWebHandler 创建一个新的WebHandler
func NewWebHandler(verbose bool) *WebHandler {
	return &WebHandler{
		entries:    make([]*TrafficEntry, 0),
		entriesMap: make(map[string]*TrafficEntry),
		verbose:    verbose,
		nextID:     1,
	}
}

// SetNewEntryCallback 设置新条目回调函数
func (h *WebHandler) SetNewEntryCallback(callback NewEntryCallback) {
	h.callbackMutex.Lock()
	defer h.callbackMutex.Unlock()
	h.newEntryCallback = callback
}

// notifyNewEntry 通知有新的流量条目
func (h *WebHandler) notifyNewEntry(entry *TrafficEntry) {
	h.callbackMutex.RLock()
	defer h.callbackMutex.RUnlock()

	if h.newEntryCallback != nil {
		h.newEntryCallback(entry)
	}
}

// generateID 生成唯一ID
func (h *WebHandler) generateID() string {
	h.idMutex.Lock()
	defer h.idMutex.Unlock()
	id := fmt.Sprintf("%d", h.nextID)
	h.nextID++
	return id
}

// GetEntries 返回所有流量条目
func (h *WebHandler) GetEntries() []*TrafficEntry {
	h.entryMutex.RLock()
	defer h.entryMutex.RUnlock()

	// 创建一个副本
	result := make([]*TrafficEntry, len(h.entries))
	copy(result, h.entries)
	return result
}

// GetEntry 根据ID获取一个特定的流量条目
func (h *WebHandler) GetEntry(id string) *TrafficEntry {
	h.entryMutex.RLock()
	defer h.entryMutex.RUnlock()

	return h.entriesMap[id]
}

// ClearEntries 清空所有流量条目
func (h *WebHandler) ClearEntries() {
	h.entryMutex.Lock()
	defer h.entryMutex.Unlock()

	h.entries = make([]*TrafficEntry, 0)
	h.entriesMap = make(map[string]*TrafficEntry)
}

// OnRequest 实现 EventHandler 接口
func (h *WebHandler) OnRequest(ctx *proxy.RequestContext) *http.Request {
	// 创建新的流量条目
	id := h.generateID()
	entry := &TrafficEntry{
		ID:             id,
		StartTime:      ctx.StartTime,
		Host:           ctx.Request.Host,
		Method:         ctx.Request.Method,
		URL:            ctx.TargetURL,
		Path:           ctx.Request.URL.Path,
		IsHTTPS:        ctx.IsHTTPS,
		IsSSE:          ctx.IsSSE,
		RequestHeaders: ctx.Request.Header.Clone(),
	}

	// 保存请求体
	if body, err := ctx.GetRequestBody(); err == nil {
		entry.RequestBody = bytes.Clone(body)
	}

	// 保存到集合中
	h.entryMutex.Lock()
	h.entries = append(h.entries, entry)
	h.entriesMap[id] = entry
	h.entryMutex.Unlock()

	// 存储ID到上下文中，以便在OnResponse中使用
	if ctx.UserData == nil {
		ctx.UserData = make(map[string]interface{})
	}
	ctx.UserData["traffic_id"] = id

	if h.verbose {
		log.Printf("[WebHandler] Captured request: %s %s", entry.Method, entry.URL)
	}

	return ctx.Request
}

// OnResponse 实现 EventHandler 接口
func (h *WebHandler) OnResponse(ctx *proxy.ResponseContext) *http.Response {
	// 从上下文中获取ID
	var id string
	if ctx.ReqCtx != nil && ctx.ReqCtx.UserData != nil {
		if idVal, ok := ctx.ReqCtx.UserData["traffic_id"]; ok {
			id = idVal.(string)
		}
	}

	if id == "" {
		if h.verbose {
			log.Println("[WebHandler] Warning: Response without request ID")
		}
		return ctx.Response
	}

	// 获取对应的流量条目
	h.entryMutex.Lock()
	defer h.entryMutex.Unlock()

	entry, ok := h.entriesMap[id]
	if !ok {
		if h.verbose {
			log.Printf("[WebHandler] Warning: No entry found for ID %s", id)
		}
		return ctx.Response
	}

	// 更新条目信息
	entry.EndTime = time.Now()
	entry.Duration = entry.EndTime.Sub(entry.StartTime).Milliseconds()
	entry.IsHTTPS = ctx.ReqCtx.IsHTTPS

	// 确保响应状态码被正确设置
	if ctx.Response != nil {
		entry.StatusCode = ctx.Response.StatusCode

		// 更新响应头信息
		entry.ResponseHeaders = ctx.Response.Header.Clone()

		// 尝试读取响应体内容
		if ctx.Response.Body != nil {
			// 创建一个副本来读取，避免消耗掉原始响应体
			bodyBytes, err := io.ReadAll(ctx.Response.Body)
			if err != nil {
				if h.verbose {
					log.Printf("[WebHandler] Error reading response body: %v", err)
				}
			} else {
				// 保存响应体内容
				entry.ResponseBody = bodyBytes
				entry.ContentSize = len(bodyBytes)

				// 重新设置响应体，供后续处理
				ctx.Response.Body = io.NopCloser(bytes.NewReader(bodyBytes))
			}
		}

		// 更新Content-Type
		contentType := ctx.Response.Header.Get("Content-Type")
		if contentType != "" {
			entry.ContentType = contentType
		}

		if h.verbose {
			log.Printf("[WebHandler] Captured response: %d for %s %s (HTTPS: %v)",
				entry.StatusCode, entry.Method, entry.URL, entry.IsHTTPS)
		}
	} else {
		// 如果响应为nil，设置一个默认值
		entry.StatusCode = 0
		entry.ContentType = ""
		entry.ContentSize = 0

		if h.verbose {
			log.Printf("[WebHandler] Captured empty response for %s %s (HTTPS: %v)",
				entry.Method, entry.URL, entry.IsHTTPS)
		}
	}

	// 直接在map中更新流量条目
	h.entriesMap[id] = entry

	// 通知有新的完整流量条目(请求+响应)
	go h.notifyNewEntry(entry)

	return ctx.Response
}

// OnError 实现 EventHandler 接口
func (h *WebHandler) OnError(err error, reqCtx *proxy.RequestContext) {
	// 从上下文中获取ID
	var id string
	if reqCtx != nil && reqCtx.UserData != nil {
		if idVal, ok := reqCtx.UserData["traffic_id"]; ok {
			id = idVal.(string)
		}
	}

	if id == "" {
		if h.verbose {
			log.Println("[WebHandler] Warning: Error without request ID")
		}
		return
	}

	// 获取对应的流量条目
	h.entryMutex.Lock()
	defer h.entryMutex.Unlock()

	entry, ok := h.entriesMap[id]
	if !ok {
		if h.verbose {
			log.Printf("[WebHandler] Warning: No entry found for ID %s", id)
		}
		return
	}

	// 更新错误信息
	entry.Error = err.Error()
	entry.EndTime = time.Now()
	entry.Duration = entry.EndTime.Sub(entry.StartTime).Milliseconds()

	if h.verbose {
		log.Printf("[WebHandler] Captured error: %v for %s %s", err, entry.Method, entry.URL)
	}
}

// OnTunnelEstablished 实现 EventHandler 接口
func (h *WebHandler) OnTunnelEstablished(host string, isIntercepted bool) {
	if h.verbose {
		log.Printf("[WebHandler] Tunnel established to %s (intercepted: %v)", host, isIntercepted)
	}
}

// OnSSE 实现 EventHandler 接口
func (h *WebHandler) OnSSE(event string, ctx *proxy.ResponseContext) {
	if h.verbose {
		log.Printf("[WebHandler] SSE event: %s", event)
	}
}
