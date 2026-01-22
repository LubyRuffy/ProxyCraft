package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/LubyRuffy/ProxyCraft/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

type timeoutErr struct{}

func (timeoutErr) Error() string   { return "read timeout" }
func (timeoutErr) Timeout() bool   { return true }
func (timeoutErr) Temporary() bool { return true }

type nonTimeoutErr struct{}

func (nonTimeoutErr) Error() string   { return "boom" }
func (nonTimeoutErr) Timeout() bool   { return false }
func (nonTimeoutErr) Temporary() bool { return false }

func TestIsTimeoutError(t *testing.T) {
	t.Run("context deadline", func(t *testing.T) {
		assert.True(t, isTimeoutError(context.DeadlineExceeded))
	})

	t.Run("net timeout", func(t *testing.T) {
		assert.True(t, isTimeoutError(timeoutErr{}))
	})

	t.Run("string timeout", func(t *testing.T) {
		assert.True(t, isTimeoutError(errors.New("Client.Timeout exceeded while awaiting headers")))
	})

	t.Run("non timeout", func(t *testing.T) {
		assert.False(t, isTimeoutError(nonTimeoutErr{}))
	})
}

func TestWebHandler_OnErrorMarksTimeout(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "traffic.db")
	handler, err := NewWebHandler(false, dbPath)
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "http://example.com/path", nil)
	reqCtx := &proxy.RequestContext{
		Request:   req,
		StartTime: time.Now(),
		TargetURL: req.URL.String(),
		UserData:  make(map[string]interface{}),
	}
	handler.OnRequest(reqCtx)

	idValue, ok := reqCtx.UserData["traffic_id"].(string)
	require.True(t, ok)

	handler.OnError(context.DeadlineExceeded, reqCtx)

	entry := handler.GetEntry(idValue)
	require.NotNil(t, entry)
	assert.True(t, entry.IsTimeout)
	assert.NotEmpty(t, entry.Error)

	handlerReloaded, err := NewWebHandler(false, dbPath)
	require.NoError(t, err)

	entryReloaded := handlerReloaded.GetEntry(idValue)
	require.NotNil(t, entryReloaded)
	assert.True(t, entryReloaded.IsTimeout)
}

func TestWebHandler_InitSQLite_AddsTimeoutColumn(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)

	legacySchema := `
CREATE TABLE IF NOT EXISTS traffic_entries (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	start_time INTEGER NOT NULL,
	end_time INTEGER,
	duration INTEGER,
	host TEXT,
	host_with_schema TEXT,
	method TEXT,
	schema TEXT,
	protocol TEXT,
	url TEXT,
	path TEXT,
	status_code INTEGER,
	content_type TEXT,
	content_size INTEGER,
	is_sse INTEGER,
	is_sse_completed INTEGER,
	is_https INTEGER,
	process_name TEXT,
	process_icon TEXT,
	request_body BLOB,
	response_body BLOB,
	request_headers BLOB,
	response_headers BLOB,
	error TEXT
);
`
	_, err = db.Exec(legacySchema)
	require.NoError(t, err)
	require.NoError(t, db.Close())

	handler, err := NewWebHandler(false, dbPath)
	require.NoError(t, err)

	req, _ := http.NewRequest("GET", "http://example.com/legacy", nil)
	reqCtx := &proxy.RequestContext{
		Request:   req,
		StartTime: time.Now(),
		TargetURL: req.URL.String(),
		UserData:  make(map[string]interface{}),
	}
	handler.OnRequest(reqCtx)

	_, ok := reqCtx.UserData["traffic_id"]
	assert.True(t, ok)
}
