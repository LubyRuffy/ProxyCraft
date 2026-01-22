package api

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/LubyRuffy/ProxyCraft/proxy/handlers"
	"github.com/stretchr/testify/assert"
)

func TestWebSocketServerFormatRequestDetailsJSON(t *testing.T) {
	ws := &WebSocketServer{}
	body := []byte(`{"hello":"world"}`)

	entry := &handlers.TrafficEntry{
		ID:             "req-1",
		Method:         "POST",
		Path:           "/test",
		Host:           "example.com",
		RequestHeaders: http.Header{"Content-Type": []string{"application/json"}},
		RequestBody:    body,
	}

	details := ws.formatRequestDetails(entry)
	headers, ok := details["headers"].(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "example.com", headers["Host"])
	assert.Equal(t, "application/json", headers["Content-Type"])
	assert.Equal(t, fmt.Sprintf("%d", len(body)), headers["Content-Length"])

	bodyValue, ok := details["body"].(map[string]interface{})
	assert.True(t, ok)
	assert.Equal(t, "world", bodyValue["hello"])
}

func TestWebSocketServerFormatResponseDetailsText(t *testing.T) {
	ws := &WebSocketServer{}
	body := []byte("ok")

	entry := &handlers.TrafficEntry{
		ID:              "resp-1",
		StatusCode:      200,
		ContentType:     "text/plain",
		ContentSize:     len(body),
		ResponseHeaders: http.Header{},
		ResponseBody:    body,
	}

	details := ws.formatResponseDetails(entry)
	headers, ok := details["headers"].(map[string]string)
	assert.True(t, ok)
	assert.Equal(t, "text/plain", headers["Content-Type"])
	assert.Equal(t, fmt.Sprintf("%d", len(body)), headers["Content-Length"])

	bodyValue, ok := details["body"].(string)
	assert.True(t, ok)
	assert.Equal(t, "ok", bodyValue)
}
