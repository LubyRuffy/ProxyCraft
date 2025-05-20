package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/LubyRuffy/ProxyCraft/proxy"
)

// CLIHandler 是一个命令行界面的事件处理器
type CLIHandler struct {
	// Verbose 是否输出详细信息
	Verbose bool

	// DumpBody 是否输出请求和响应的主体
	DumpBody bool

	// 计数器
	RequestCount  int
	ResponseCount int
	ErrorCount    int
	TunnelCount   int
	SSECount      int
}

// NewCLIHandler 创建一个新的命令行事件处理器
func NewCLIHandler(verbose, dumpBody bool) *CLIHandler {
	return &CLIHandler{
		Verbose:  verbose,
		DumpBody: dumpBody,
	}
}

// OnRequest 实现 EventHandler 接口
func (h *CLIHandler) OnRequest(ctx *proxy.RequestContext) *http.Request {
	h.RequestCount++

	if h.Verbose {
		fmt.Printf("[REQ] #%d %s %s\n", h.RequestCount, ctx.Request.Method, ctx.TargetURL)

		// 输出请求头
		fmt.Println("[REQ] Headers:")
		for key, values := range ctx.Request.Header {
			for _, value := range values {
				fmt.Printf("[REQ]   %s: %s\n", key, value)
			}
		}

		// 如果需要输出主体
		if h.DumpBody {
			body, err := ctx.GetRequestBody()
			if err != nil {
				fmt.Printf("[REQ] Error reading body: %v\n", err)
			} else if len(body) > 0 {
				if len(body) > 1024 {
					fmt.Printf("[REQ] Body (%d bytes, showing first 1024):\n%s...\n", len(body), body[:1024])
				} else {
					fmt.Printf("[REQ] Body (%d bytes):\n%s\n", len(body), body)
				}
			}
		}
	} else {
		fmt.Printf("[REQ] #%d %s %s\n", h.RequestCount, ctx.Request.Method, ctx.TargetURL)
	}

	return ctx.Request
}

// OnResponse 实现 EventHandler 接口
func (h *CLIHandler) OnResponse(ctx *proxy.ResponseContext) *http.Response {
	h.ResponseCount++

	if h.Verbose {
		fmt.Printf("[RES] #%d %s %s -> %d %s (took %dms)\n",
			h.ResponseCount,
			ctx.ReqCtx.Request.Method,
			ctx.ReqCtx.TargetURL,
			ctx.Response.StatusCode,
			ctx.Response.Header.Get("Content-Type"),
			ctx.TimeTaken.Milliseconds())

		// 输出响应头
		fmt.Println("[RES] Headers:")
		for key, values := range ctx.Response.Header {
			for _, value := range values {
				fmt.Printf("[RES]   %s: %s\n", key, value)
			}
		}

		// 如果需要输出主体，且不是SSE
		if h.DumpBody && !ctx.IsSSE {
			body, err := ctx.GetResponseBody()
			if err != nil {
				fmt.Printf("[RES] Error reading body: %v\n", err)
			} else if len(body) > 0 {
				if len(body) > 1024 {
					fmt.Printf("[RES] Body (%d bytes, showing first 1024):\n%s...\n", len(body), body[:1024])
				} else {
					fmt.Printf("[RES] Body (%d bytes):\n%s\n", len(body), body)
				}
			}
		}
	} else {
		fmt.Printf("[RES] #%d %s %s -> %d %s (took %dms)\n",
			h.ResponseCount,
			ctx.ReqCtx.Request.Method,
			ctx.ReqCtx.TargetURL,
			ctx.Response.StatusCode,
			ctx.Response.Header.Get("Content-Type"),
			ctx.TimeTaken.Milliseconds())
	}

	return ctx.Response
}

// OnError 实现 EventHandler 接口
func (h *CLIHandler) OnError(err error, reqCtx *proxy.RequestContext) {
	h.ErrorCount++

	if reqCtx != nil {
		log.Printf("[ERR] #%d %s %s: %v", h.ErrorCount, reqCtx.Request.Method, reqCtx.TargetURL, err)
	} else {
		log.Printf("[ERR] #%d: %v", h.ErrorCount, err)
	}
}

// OnTunnelEstablished 实现 EventHandler 接口
func (h *CLIHandler) OnTunnelEstablished(host string, isIntercepted bool) {
	h.TunnelCount++
	log.Printf("[TUN] #%d 与 %s 的隧道已建立", h.TunnelCount, host)
}

// OnSSE 实现 EventHandler 接口
func (h *CLIHandler) OnSSE(event string, ctx *proxy.ResponseContext) {
	h.SSECount++

	if h.Verbose {
		fmt.Printf("[SSE] #%d %s\n", h.SSECount, event)
	}
}

// GetStats 获取处理器的统计信息
func (h *CLIHandler) GetStats() string {
	return fmt.Sprintf(
		"请求: %d, 响应: %d, 错误: %d, 隧道: %d, SSE事件: %d",
		h.RequestCount,
		h.ResponseCount,
		h.ErrorCount,
		h.TunnelCount,
		h.SSECount,
	)
}

// 统计报告器，定期输出统计信息
type StatsReporter struct {
	Handler   *CLIHandler
	Running   bool
	Interval  time.Duration
	StopChan  chan struct{}
	StartTime time.Time
}

// NewStatsReporter 创建一个新的统计报告器
func NewStatsReporter(handler *CLIHandler, interval time.Duration) *StatsReporter {
	return &StatsReporter{
		Handler:   handler,
		Interval:  interval,
		StopChan:  make(chan struct{}),
		StartTime: time.Now(),
	}
}

// Start 开始定期报告
func (r *StatsReporter) Start() {
	if r.Running {
		return
	}

	r.Running = true
	r.StartTime = time.Now()

	go func() {
		ticker := time.NewTicker(r.Interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				r.Report()
			case <-r.StopChan:
				return
			}
		}
	}()
}

// Stop 停止报告
func (r *StatsReporter) Stop() {
	if !r.Running {
		return
	}

	r.StopChan <- struct{}{}
	r.Running = false
	r.Report() // 输出最终报告
}

// Report 输出一次报告
func (r *StatsReporter) Report() {
	runningTime := time.Since(r.StartTime)
	fmt.Printf("\n--- 统计报告（运行时间: %s）---\n", runningTime.Round(time.Second))
	fmt.Println(r.Handler.GetStats())
	fmt.Printf("QPS: %.2f, 平均响应时间: %.2fms\n",
		float64(r.Handler.RequestCount)/runningTime.Seconds(),
		float64(r.Handler.ResponseCount)/float64(r.Handler.RequestCount)*1000)
	fmt.Print("--------------------------------\n")
}
