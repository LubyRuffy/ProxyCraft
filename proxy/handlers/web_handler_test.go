package handlers

import (
	"bytes"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/LubyRuffy/ProxyCraft/proxy"
	"github.com/stretchr/testify/assert"
)

// TestWebHandler_GetEntries_Concurrency 测试GetEntries在高并发场景下的性能和稳定性
func TestWebHandler_GetEntries_Concurrency(t *testing.T) {
	// 创建一个WebHandler实例
	handler := NewWebHandler(false)

	// 添加大量测试数据
	entriesCount := 500
	for i := 0; i < entriesCount; i++ {
		req, _ := http.NewRequest("GET", "http://example.com/path", nil)
		reqCtx := &proxy.RequestContext{
			Request:   req,
			StartTime: time.Now(),
			TargetURL: "http://example.com/path",
			UserData:  make(map[string]interface{}),
		}

		handler.OnRequest(reqCtx)

		// 创建一个响应
		resp := &http.Response{
			StatusCode: 200,
			Header:     http.Header{},
			Body:       io.NopCloser(bytes.NewBufferString("test response body")),
		}

		respCtx := &proxy.ResponseContext{
			Response: resp,
			ReqCtx:   reqCtx,
		}

		handler.OnResponse(respCtx)
	}

	// 确认数据已添加
	assert.Equal(t, entriesCount, len(handler.entries))

	// 并发调用GetEntries
	var wg sync.WaitGroup
	concurrencyLevel := 10  // 10个并发goroutine
	callsPerGoroutine := 10 // 每个goroutine调用10次

	// 用于记录每个goroutine执行时间的通道
	timeResults := make(chan time.Duration, concurrencyLevel)

	t.Log("开始并发测试GetEntries")
	for i := 0; i < concurrencyLevel; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			start := time.Now()
			for j := 0; j < callsPerGoroutine; j++ {
				entries := handler.GetEntries()
				// 验证返回的条目数量正确（应该是1000或者全部条目数）
				expected := entriesCount
				if expected > 1000 {
					expected = 1000
				}
				if len(entries) != expected {
					t.Errorf("Goroutine %d, call %d: expected %d entries, got %d", id, j, expected, len(entries))
				}
			}
			elapsed := time.Since(start)
			timeResults <- elapsed
		}(i)
	}

	// 等待所有goroutine完成
	wg.Wait()
	close(timeResults)

	// 统计执行时间
	var totalTime time.Duration
	var maxTime time.Duration
	count := 0

	for elapsed := range timeResults {
		totalTime += elapsed
		if elapsed > maxTime {
			maxTime = elapsed
		}
		count++
	}

	avgTime := totalTime / time.Duration(count)
	t.Logf("GetEntries并发测试结果: 平均时间 %v, 最长时间 %v", avgTime, maxTime)

	// 如果最长时间超过1秒，发出警告
	if maxTime > time.Second {
		t.Logf("警告: GetEntries最长执行时间(%v)超过1秒，可能存在性能问题", maxTime)
	}
}
