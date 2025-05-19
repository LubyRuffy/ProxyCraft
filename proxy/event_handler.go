package proxy

import (
	"bytes"
	"io"
	"net/http"
	"time"
)

// EventHandler 定义了代理处理不同事件的接口
type EventHandler interface {
	// OnRequest 在收到请求时调用
	OnRequest(ctx *RequestContext) *http.Request

	// OnResponse 在收到响应时调用
	OnResponse(ctx *ResponseContext) *http.Response

	// OnError 在处理过程中发生错误时调用
	OnError(err error, reqCtx *RequestContext)

	// OnTunnelEstablished 在HTTPS隧道建立时调用
	OnTunnelEstablished(host string, isIntercepted bool)

	// OnSSE 在处理服务器发送事件流时调用
	OnSSE(event string, ctx *ResponseContext)
}

// RequestContext 包含请求的上下文信息
type RequestContext struct {
	// 原始请求
	Request *http.Request

	// 请求的开始时间
	StartTime time.Time

	// IsSSE 表示这是否可能是一个SSE请求
	IsSSE bool

	// IsHTTPS 表示这是否是HTTPS请求
	IsHTTPS bool

	// TargetURL 表示请求的目标URL
	TargetURL string

	// 用于保存上下文的自定义数据
	UserData map[string]interface{}
}

// GetRequestBody 获取请求体的内容，同时保持请求体可以再次被读取
func (ctx *RequestContext) GetRequestBody() ([]byte, error) {
	if ctx.Request == nil || ctx.Request.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		return nil, err
	}

	// 重置请求体，使其可以再次被读取
	ctx.Request.Body = io.NopCloser(bytes.NewBuffer(body))

	return body, nil
}

// ResponseContext 包含响应的上下文信息
type ResponseContext struct {
	// 原始请求上下文
	ReqCtx *RequestContext

	// 原始响应
	Response *http.Response

	// 响应耗时
	TimeTaken time.Duration

	// IsSSE 表示这是否是一个SSE响应
	IsSSE bool

	// 用于保存上下文的自定义数据
	UserData map[string]interface{}
}

// GetResponseBody 获取响应体的内容，同时保持响应体可以再次被读取
func (ctx *ResponseContext) GetResponseBody() ([]byte, error) {
	if ctx.Response == nil || ctx.Response.Body == nil {
		return nil, nil
	}

	body, err := io.ReadAll(ctx.Response.Body)
	if err != nil {
		return nil, err
	}

	// 重置响应体，使其可以再次被读取
	ctx.Response.Body = io.NopCloser(bytes.NewBuffer(body))

	return body, nil
}

// NoOpEventHandler 提供一个默认的空实现，方便只重写部分方法
type NoOpEventHandler struct{}

// OnRequest 实现 EventHandler 接口
func (h *NoOpEventHandler) OnRequest(ctx *RequestContext) *http.Request {
	return ctx.Request
}

// OnResponse 实现 EventHandler 接口
func (h *NoOpEventHandler) OnResponse(ctx *ResponseContext) *http.Response {
	return ctx.Response
}

// OnError 实现 EventHandler 接口
func (h *NoOpEventHandler) OnError(err error, reqCtx *RequestContext) {}

// OnTunnelEstablished 实现 EventHandler 接口
func (h *NoOpEventHandler) OnTunnelEstablished(host string, isIntercepted bool) {}

// OnSSE 实现 EventHandler 接口
func (h *NoOpEventHandler) OnSSE(event string, ctx *ResponseContext) {}

// MultiEventHandler 允许注册多个事件处理器
type MultiEventHandler struct {
	handlers []EventHandler
}

// NewMultiEventHandler 创建一个新的多事件处理器
func NewMultiEventHandler(handlers ...EventHandler) *MultiEventHandler {
	return &MultiEventHandler{
		handlers: handlers,
	}
}

// AddHandler 添加一个事件处理器
func (m *MultiEventHandler) AddHandler(handler EventHandler) {
	m.handlers = append(m.handlers, handler)
}

// OnRequest 实现 EventHandler 接口，调用所有处理器
func (m *MultiEventHandler) OnRequest(ctx *RequestContext) *http.Request {
	req := ctx.Request
	for _, handler := range m.handlers {
		if modifiedReq := handler.OnRequest(ctx); modifiedReq != nil {
			req = modifiedReq
			ctx.Request = req
		}
	}
	return req
}

// OnResponse 实现 EventHandler 接口，调用所有处理器
func (m *MultiEventHandler) OnResponse(ctx *ResponseContext) *http.Response {
	resp := ctx.Response
	for _, handler := range m.handlers {
		if modifiedResp := handler.OnResponse(ctx); modifiedResp != nil {
			resp = modifiedResp
			ctx.Response = resp
		}
	}
	return resp
}

// OnError 实现 EventHandler 接口，调用所有处理器
func (m *MultiEventHandler) OnError(err error, reqCtx *RequestContext) {
	for _, handler := range m.handlers {
		handler.OnError(err, reqCtx)
	}
}

// OnTunnelEstablished 实现 EventHandler 接口，调用所有处理器
func (m *MultiEventHandler) OnTunnelEstablished(host string, isIntercepted bool) {
	for _, handler := range m.handlers {
		handler.OnTunnelEstablished(host, isIntercepted)
	}
}

// OnSSE 实现 EventHandler 接口，调用所有处理器
func (m *MultiEventHandler) OnSSE(event string, ctx *ResponseContext) {
	for _, handler := range m.handlers {
		handler.OnSSE(event, ctx)
	}
}
