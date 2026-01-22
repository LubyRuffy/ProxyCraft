package api

import (
	"net/http"
	"testing"

	"github.com/LubyRuffy/ProxyCraft/proxy/handlers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractLLMOpenAIChat(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "api.openai.com",
		Path: "/v1/chat/completions",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[
				{"role":"system","content":"You are a bot."},
				{"role":"user","content":"Hello"}
			],
			"tools":[{"type":"function","function":{"name":"search","parameters":{"type":"object"}}}]
		}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		ResponseBody: []byte(`{
			"id":"resp-1",
			"model":"gpt-4o-mini",
			"choices":[{
				"message":{
					"content":"Hi there!",
					"reasoning":"internal",
					"tool_calls":[{"id":"call_1","type":"function","function":{"name":"search","arguments":"{\"q\":\"hi\"}"}}]
				}
			}]
		}`),
	}

	info := ExtractLLM(entry, true, true)
	require.NotNil(t, info)
	assert.Equal(t, "openai", info.Provider)
	assert.Equal(t, "gpt-4o-mini", info.Model)
	require.NotNil(t, info.Request)
	assert.Contains(t, info.Request.Prompt, "**system**")
	assert.Contains(t, info.Request.Prompt, "Hello")
	assert.NotNil(t, info.Request.Tools)
	require.NotNil(t, info.Response)
	assert.Equal(t, "Hi there!", info.Response.Content)
	assert.NotEmpty(t, info.Response.ToolCalls)
	assert.Equal(t, "internal", info.Response.Reasoning)
}

func TestExtractLLMOpenAISSE(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "api.openai.com",
		Path: "/v1/chat/completions",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[{"role":"user","content":"Hello"}],
			"stream":true
		}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		ResponseBody: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n" +
			"data: [DONE]\n\n"),
		IsSSE: true,
	}

	info := ExtractLLM(entry, false, true)
	require.NotNil(t, info)
	assert.True(t, info.Streaming)
	require.NotNil(t, info.Response)
	assert.Equal(t, "Hello world", info.Response.Content)
}

func TestExtractLLMOpenAISSEWithoutBlankLines(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "api.openai.com",
		Path: "/v1/chat/completions",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{
			"model":"gpt-4o-mini",
			"messages":[{"role":"user","content":"Hello"}],
			"stream":true
		}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		ResponseBody: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n" +
			"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n" +
			"data: [DONE]\n"),
		IsSSE: true,
	}

	info := ExtractLLM(entry, false, true)
	require.NotNil(t, info)
	require.NotNil(t, info.Response)
	assert.Equal(t, "Hello world", info.Response.Content)
}

func TestExtractLLMResponsesSSE(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "chatgpt.com",
		Path: "/backend-api/codex/responses",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{"input":[{"role":"user","content":"hi"}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		ResponseBody: []byte("data: {\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n\n" +
			"data: {\"type\":\"response.output_text.delta\",\"delta\":\" world\"}\n\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-test\"}}\n\n"),
		IsSSE: true,
	}

	info := ExtractLLM(entry, false, true)
	require.NotNil(t, info)
	assert.Equal(t, "openai-compatible", info.Provider)
	assert.Equal(t, "gpt-test", info.Model)
	require.NotNil(t, info.Response)
	assert.Equal(t, "Hello world", info.Response.Content)
}

func TestExtractLLMResponsesSSEWithEventLines(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "chatgpt.com",
		Path: "/backend-api/codex/responses",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{"input":[{"role":"user","content":"hi"}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		ResponseBody: []byte("event:response.output_text.delta\n" +
			"data:{\"type\":\"response.output_text.delta\",\"delta\":\"Hello\"}\n" +
			"event:response.output_text.delta\n" +
			"data:{\"type\":\"response.output_text.delta\",\"delta\":\" world\"}\n"),
		IsSSE: true,
	}

	info := ExtractLLM(entry, false, true)
	require.NotNil(t, info)
	require.NotNil(t, info.Response)
	assert.Equal(t, "Hello world", info.Response.Content)
}

func TestExtractLLMResponsesSSEOutputItemAdded(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "chatgpt.com",
		Path: "/backend-api/codex/responses",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{"input":[{"role":"user","content":"hi"}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		ResponseBody: []byte("data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"call_1\",\"type\":\"web_search_call\",\"query\":\"hello\"}}\n\n" +
			"data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"call_2\",\"type\":\"tool_call\",\"name\":\"search\",\"arguments\":\"{\\\"q\\\":\\\"hi\\\"}\"}}\n\n" +
			"data: [DONE]\n\n"),
		IsSSE: true,
	}

	info := ExtractLLM(entry, false, true)
	require.NotNil(t, info)
	require.NotNil(t, info.Response)
	require.NotNil(t, info.Response.ToolCalls)
	calls, ok := info.Response.ToolCalls.([]interface{})
	require.True(t, ok)
	assert.GreaterOrEqual(t, len(calls), 2)
}

func TestExtractLLMResponsesSSEContentPartAdded(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "chatgpt.com",
		Path: "/backend-api/codex/responses",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{"input":[{"role":"user","content":"hi"}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		ResponseBody: []byte("data: {\"type\":\"response.content_part.added\",\"item_id\":\"msg_1\",\"content_index\":0,\"part\":{\"type\":\"output_text\",\"text\":\"Hello\"}}\n\n" +
			"data: {\"type\":\"response.completed\",\"response\":{\"model\":\"gpt-test\"}}\n\n"),
		IsSSE: true,
	}

	info := ExtractLLM(entry, false, true)
	require.NotNil(t, info)
	require.NotNil(t, info.Response)
	assert.Equal(t, "Hello", info.Response.Content)
}

func TestExtractLLMResponsesSSEOutputTextDoneNoDuplicate(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "chatgpt.com",
		Path: "/backend-api/codex/responses",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{"input":[{"role":"user","content":"hi"}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		ResponseBody: []byte("data: {\"type\":\"response.output_text.done\",\"item_id\":\"msg_1\",\"content_index\":0,\"text\":\"Hello world\"}\n\n" +
			"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"msg_1\",\"type\":\"message\",\"content\":[{\"type\":\"output_text\",\"text\":\"Hello world\"}]}}\n\n"),
		IsSSE: true,
	}

	info := ExtractLLM(entry, false, true)
	require.NotNil(t, info)
	require.NotNil(t, info.Response)
	assert.Equal(t, "Hello world", info.Response.Content)
}

func TestExtractLLMResponsesSSEToolCallUpsert(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "chatgpt.com",
		Path: "/backend-api/codex/responses",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{"input":[{"role":"user","content":"hi"}]}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"text/event-stream"},
		},
		ResponseBody: []byte("data: {\"type\":\"response.output_item.added\",\"item\":{\"id\":\"call_1\",\"type\":\"web_search_call\",\"status\":\"in_progress\"}}\n\n" +
			"data: {\"type\":\"response.output_item.done\",\"item\":{\"id\":\"call_1\",\"type\":\"web_search_call\",\"status\":\"completed\",\"action\":{\"type\":\"search\",\"query\":\"hi\"}}}\n\n"),
		IsSSE: true,
	}

	info := ExtractLLM(entry, false, true)
	require.NotNil(t, info)
	require.NotNil(t, info.Response)
	require.NotNil(t, info.Response.ToolCalls)

	calls, ok := info.Response.ToolCalls.([]interface{})
	require.True(t, ok)
	require.Len(t, calls, 1)
	call, ok := calls[0].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "completed", call["status"])
	action, ok := call["action"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "hi", action["query"])
}

func TestExtractLLMClaude(t *testing.T) {
	entry := &handlers.TrafficEntry{
		Host: "api.anthropic.com",
		Path: "/v1/messages",
		RequestHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		RequestBody: []byte(`{
			"model":"claude-3-5-sonnet-20240620",
			"system":"You are helpful.",
			"messages":[{"role":"user","content":[{"type":"text","text":"Summarize this."}]}],
			"tools":[{"name":"lookup","description":"Lookup docs."}]
		}`),
		ResponseHeaders: http.Header{
			"Content-Type": []string{"application/json"},
		},
		ResponseBody: []byte(`{
			"content":[
				{"type":"text","text":"Summary result."},
				{"type":"tool_use","id":"toolu_1","name":"lookup","input":{"q":"test"}},
				{"type":"thinking","thinking":"Reasoning chain"}
			]
		}`),
	}

	info := ExtractLLM(entry, true, true)
	require.NotNil(t, info)
	assert.Equal(t, "claude", info.Provider)
	assert.Equal(t, "claude-3-5-sonnet-20240620", info.Model)
	require.NotNil(t, info.Request)
	assert.Contains(t, info.Request.Prompt, "**system**")
	assert.Contains(t, info.Request.Prompt, "Summarize this.")
	assert.NotNil(t, info.Request.Tools)
	require.NotNil(t, info.Response)
	assert.Equal(t, "Summary result.", info.Response.Content)
	assert.NotEmpty(t, info.Response.ToolCalls)
	assert.Equal(t, "Reasoning chain", info.Response.Reasoning)
}
