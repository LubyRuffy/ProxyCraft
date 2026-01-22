package handlers

import (
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/LubyRuffy/ProxyCraft/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebHandler_OnSSE_AppendsBlankLine(t *testing.T) {
	handler, err := NewWebHandler(false, filepath.Join(t.TempDir(), "traffic.db"))
	require.NoError(t, err)

	req, err := http.NewRequest("GET", "http://example.com/sse", nil)
	require.NoError(t, err)

	reqCtx := &proxy.RequestContext{
		Request:   req,
		StartTime: time.Now(),
		TargetURL: req.URL.String(),
		UserData:  make(map[string]interface{}),
	}
	handler.OnRequest(reqCtx)

	id, ok := reqCtx.UserData["traffic_id"].(string)
	require.True(t, ok)

	respCtx := &proxy.ResponseContext{
		ReqCtx: reqCtx,
	}

	event := "event:response.created\ndata:{\"type\":\"response.created\"}"
	handler.OnSSE(event, respCtx)

	entry := handler.GetEntry(id)
	require.NotNil(t, entry)
	assert.Equal(t, event+"\n\n", string(entry.ResponseBody))
}
