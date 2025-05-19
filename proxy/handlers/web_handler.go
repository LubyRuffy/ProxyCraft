package handlers

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
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
	maxEntries       int                      // 最大条目数
}

// NewWebHandler 创建一个新的WebHandler
func NewWebHandler(verbose bool) *WebHandler {
	handler := &WebHandler{
		entries:    make([]*TrafficEntry, 0),
		entriesMap: make(map[string]*TrafficEntry),
		verbose:    verbose,
		nextID:     1,
		maxEntries: 2000, // 默认最多保存2000条记录
	}

	// 启动自动清理任务
	go handler.startCleanupTask()

	return handler
}

// 自动清理任务
func (h *WebHandler) startCleanupTask() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		h.cleanupOldEntries()
	}
}

// 清理旧的条目
func (h *WebHandler) cleanupOldEntries() {
	h.entryMutex.Lock()
	defer h.entryMutex.Unlock()

	entriesLen := len(h.entries)
	if entriesLen <= h.maxEntries {
		return
	}

	// 计算需要删除的条目数量
	deleteCount := entriesLen - h.maxEntries

	if h.verbose {
		log.Printf("[WebHandler] 清理 %d 条旧流量记录，当前总数: %d", deleteCount, entriesLen)
	}

	// 删除最旧的条目
	for i := 0; i < deleteCount; i++ {
		// 从map中删除
		delete(h.entriesMap, h.entries[i].ID)
	}

	// 更新切片
	h.entries = h.entries[deleteCount:]
}

// SetNewEntryCallback 设置新条目回调函数
func (h *WebHandler) SetNewEntryCallback(callback NewEntryCallback) {
	h.callbackMutex.Lock()
	h.newEntryCallback = callback
	h.callbackMutex.Unlock()

	if h.verbose {
		if callback == nil {
			log.Printf("[WebHandler] 清除新条目回调函数")
		} else {
			log.Printf("[WebHandler] 设置新条目回调函数")
		}
	}
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
	startTime := time.Now()

	h.entryMutex.RLock()

	// 获取entries的长度
	entriesLen := len(h.entries)
	if entriesLen == 0 {
		h.entryMutex.RUnlock()
		return []*TrafficEntry{}
	}

	// 如果条目数量过大，只返回最新的1000条
	maxEntries := 1000
	startIndex := 0
	if entriesLen > maxEntries {
		startIndex = entriesLen - maxEntries
	}

	// 创建结果数组，直接在锁内生成结果
	resultLen := entriesLen - startIndex
	result := make([]*TrafficEntry, resultLen)

	// 生成轻量级条目，不包含大数据字段
	for i := 0; i < resultLen; i++ {
		srcEntry := h.entries[startIndex+i]

		// 创建一个不包含请求体和响应体等大数据的新条目
		result[i] = &TrafficEntry{
			ID:          srcEntry.ID,
			StartTime:   srcEntry.StartTime,
			EndTime:     srcEntry.EndTime,
			Duration:    srcEntry.Duration,
			Host:        srcEntry.Host,
			Method:      srcEntry.Method,
			URL:         srcEntry.URL,
			Path:        srcEntry.Path,
			StatusCode:  srcEntry.StatusCode,
			ContentType: srcEntry.ContentType,
			ContentSize: srcEntry.ContentSize,
			IsSSE:       srcEntry.IsSSE,
			IsHTTPS:     srcEntry.IsHTTPS,
			Error:       srcEntry.Error,
		}
	}

	// 释放锁
	h.entryMutex.RUnlock()

	// 记录性能日志
	elapsed := time.Since(startTime)
	if elapsed > 100*time.Millisecond {
		log.Printf("[WebHandler] GetEntries: 返回 %d 条记录耗时 %v", resultLen, elapsed)
	}

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
	// 检查是否需要清理
	entriesLen := len(h.entries)
	if entriesLen > h.maxEntries {
		go h.cleanupOldEntries()
	}

	// 生成ID并准备数据
	id := h.generateID()

	// 准备新的流量条目，尽可能在锁外完成
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
		// 如果请求体过大，只保存部分
		if len(body) > 10*1024*1024 { // 超过10MB
			if h.verbose {
				log.Printf("[WebHandler] 请求体过大 (%d bytes)，只保存前10MB", len(body))
			}
			entry.RequestBody = append(body[:10*1024*1024], []byte("... [截断过大的请求体] ...")...)
		} else {
			entry.RequestBody = bytes.Clone(body)
		}
	}

	// 数据准备好后，再获取锁添加条目
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

	// 首先获取对应的entry引用，但不修改数据
	h.entryMutex.RLock()
	entry, ok := h.entriesMap[id]
	h.entryMutex.RUnlock()

	if !ok {
		if h.verbose {
			log.Printf("[WebHandler] Warning: No entry found for ID %s", id)
		}
		return ctx.Response
	}

	// 准备更新的数据
	endTime := time.Now()
	duration := endTime.Sub(entry.StartTime).Milliseconds()
	isHTTPS := ctx.ReqCtx.IsHTTPS

	var statusCode int
	var contentType string
	var contentSize int
	var responseHeaders http.Header
	var responseBody []byte

	// 处理响应数据
	if ctx.Response != nil {
		statusCode = ctx.Response.StatusCode
		responseHeaders = ctx.Response.Header.Clone()

		// 检查是否是SSE响应，对SSE响应做特殊处理
		if ctx.IsSSE {
			// 对于SSE响应，我们不读取响应体，因为它是流式的
			contentType = "text/event-stream"
			contentSize = -1 // 表示大小未知
			if h.verbose {
				log.Printf("[WebHandler] Skipping body read for SSE response: %s", entry.URL)
			}
		} else {
			// 非SSE响应，读取响应体
			if ctx.Response.Body != nil {
				// 设置最大读取限制
				maxSize := 10 * 1024 * 1024 // 10MB
				limitReader := io.LimitReader(ctx.Response.Body, int64(maxSize))

				// 读取响应体
				var err error
				bodyBytes, err := io.ReadAll(limitReader)
				if err != nil {
					if h.verbose {
						log.Printf("[WebHandler] Error reading response body: %v", err)
					}
				} else {
					actualSize := len(bodyBytes)

					// 检查是否截断了
					if actualSize >= maxSize {
						// 读取了限制大小，可能有更多未读取的数据
						if h.verbose {
							log.Printf("[WebHandler] 响应体过大 (>=%d bytes)，已截断", maxSize)
						}

						// 添加截断提示
						bodyBytes = append(bodyBytes, []byte("... [截断过大的响应体] ...")...)

						// 尝试通过Content-Length头判断实际大小
						if contentLenStr := ctx.Response.Header.Get("Content-Length"); contentLenStr != "" {
							if contentLen, err := strconv.Atoi(contentLenStr); err == nil {
								contentSize = contentLen
							} else {
								contentSize = maxSize
							}
						} else {
							contentSize = maxSize
						}
					} else {
						contentSize = actualSize
					}

					responseBody = bodyBytes

					// 重新设置响应体，供后续处理
					ctx.Response.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}

			// 更新Content-Type
			contentType = ctx.Response.Header.Get("Content-Type")
		}

		if h.verbose {
			log.Printf("[WebHandler] Captured response: %d for %s %s (HTTPS: %v, Size: %d bytes)",
				statusCode, entry.Method, entry.URL, isHTTPS, contentSize)
		}
	} else {
		// 如果响应为nil，设置默认值
		statusCode = 0
		contentType = ""
		contentSize = 0

		if h.verbose {
			log.Printf("[WebHandler] Captured empty response for %s %s (HTTPS: %v)",
				entry.Method, entry.URL, isHTTPS)
		}
	}

	// 所有数据准备好后，再获取写锁更新条目
	h.entryMutex.Lock()

	// 再次检查entry是否存在，避免竞态条件
	entry, stillExists := h.entriesMap[id]
	if !stillExists {
		h.entryMutex.Unlock()
		if h.verbose {
			log.Printf("[WebHandler] Entry disappeared during processing, ID %s", id)
		}
		return ctx.Response
	}

	// 更新条目信息
	entry.EndTime = endTime
	entry.Duration = duration
	entry.IsHTTPS = isHTTPS
	entry.StatusCode = statusCode
	entry.ContentType = contentType
	entry.ContentSize = contentSize
	if responseHeaders != nil {
		entry.ResponseHeaders = responseHeaders
	}
	if responseBody != nil {
		entry.ResponseBody = responseBody
	}

	// 释放锁
	h.entryMutex.Unlock()

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

	// 首先获取对应的entry引用，但不修改数据
	h.entryMutex.RLock()
	entry, ok := h.entriesMap[id]
	h.entryMutex.RUnlock()

	if !ok {
		if h.verbose {
			log.Printf("[WebHandler] Warning: No entry found for ID %s", id)
		}
		return
	}

	// 准备更新的数据
	errorMsg := err.Error()
	endTime := time.Now()
	duration := endTime.Sub(entry.StartTime).Milliseconds()

	// 获取写锁并更新
	h.entryMutex.Lock()

	// 再次检查entry是否存在
	entry, stillExists := h.entriesMap[id]
	if !stillExists {
		h.entryMutex.Unlock()
		if h.verbose {
			log.Printf("[WebHandler] Entry disappeared during error processing, ID %s", id)
		}
		return
	}

	// 更新错误信息
	entry.Error = errorMsg
	entry.EndTime = endTime
	entry.Duration = duration

	h.entryMutex.Unlock()

	if h.verbose {
		log.Printf("[WebHandler] Captured error: %v for %s %s", err, entry.Method, entry.URL)
	}

	// 通知有新的条目更新
	go h.notifyNewEntry(entry)
}

// OnTunnelEstablished 实现 EventHandler 接口
func (h *WebHandler) OnTunnelEstablished(host string, isIntercepted bool) {
	if h.verbose {
		log.Printf("[WebHandler] Tunnel established to %s (intercepted: %v)", host, isIntercepted)
	}
}

// OnSSE 实现 EventHandler 接口
func (h *WebHandler) OnSSE(event string, ctx *proxy.ResponseContext) {
	// 从上下文中获取ID
	var id string
	if ctx != nil && ctx.ReqCtx != nil && ctx.ReqCtx.UserData != nil {
		if idVal, ok := ctx.ReqCtx.UserData["traffic_id"]; ok {
			id = idVal.(string)
		}
	}

	if id == "" {
		if h.verbose {
			log.Println("[WebHandler] Warning: SSE event without request ID")
		}
		return
	}

	// 首先获取对应的entry引用，但不修改数据
	h.entryMutex.RLock()
	entry, ok := h.entriesMap[id]
	h.entryMutex.RUnlock()

	if !ok {
		if h.verbose {
			log.Printf("[WebHandler] Warning: No entry found for SSE event, ID %s", id)
		}
		return
	}

	// 准备更新的数据
	endTime := time.Now()
	duration := endTime.Sub(entry.StartTime).Milliseconds()
	eventBytes := []byte(event + "\n")

	// 获取写锁并更新
	h.entryMutex.Lock()

	// 再次检查entry是否存在
	entry, stillExists := h.entriesMap[id]
	if !stillExists {
		h.entryMutex.Unlock()
		if h.verbose {
			log.Printf("[WebHandler] Entry disappeared during SSE processing, ID %s", id)
		}
		return
	}

	// 动态更新ResponseBody，累积记录SSE事件内容
	// 如果之前没有收到过SSE内容，初始化ResponseBody
	if entry.ResponseBody == nil || entry.ContentSize <= 0 {
		entry.ResponseBody = eventBytes
	} else {
		// 否则追加新的事件内容，但限制总体大小
		maxSseSize := 1024 * 1024 // 1MB 限制
		if len(entry.ResponseBody)+len(eventBytes) > maxSseSize {
			// 如果添加新的事件会超过大小限制，先截断现有内容
			entry.ResponseBody = append(
				entry.ResponseBody[:maxSseSize/2],
				[]byte("\n... [截断部分SSE事件] ...\n")...,
			)
		}
		entry.ResponseBody = append(entry.ResponseBody, eventBytes...)
	}

	// 更新其他字段
	entry.ContentSize = len(entry.ResponseBody)
	entry.ContentType = "text/event-stream"
	entry.EndTime = endTime
	entry.Duration = duration

	h.entryMutex.Unlock()

	// 通知有新的完整流量条目(请求+响应)
	go h.notifyNewEntry(entry)

	if h.verbose {
		log.Printf("[WebHandler] SSE event: %s, updated entry ID %s, total size %d bytes",
			event, id, entry.ContentSize)
	}
}
