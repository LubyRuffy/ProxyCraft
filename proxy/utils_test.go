package proxy

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestRealUtilsFunctions contains the tests for utility functions in utils.go
func TestIsBinaryContent(t *testing.T) {
	// 测试二进制内容识别
	binaryData := []byte{0x00, 0x01, 0x02, 0x03}
	textData := []byte("This is a text content") // 使用ASCII文本而不是中文

	// 基于Content-Type的检测
	assert.True(t, isBinaryContent([]byte{}, "image/png"))
	assert.True(t, isBinaryContent([]byte{}, "application/pdf"))
	assert.False(t, isBinaryContent([]byte{}, "text/plain"))
	assert.False(t, isBinaryContent([]byte{}, "application/json"))

	// 基于内容的检测
	assert.True(t, isBinaryContent(binaryData, ""))
	assert.False(t, isBinaryContent(textData, ""))
}

func TestReadAndRestoreBody(t *testing.T) {
	// 创建测试体
	testBody := io.NopCloser(strings.NewReader("test body"))
	bodySlot := &testBody

	// 读取并恢复
	bodyBytes, err := readAndRestoreBody(bodySlot, -1)
	assert.NoError(t, err)
	assert.Equal(t, "test body", string(bodyBytes))

	// 检查是否恢复了Body
	restoredContent, err := io.ReadAll(*bodySlot)
	assert.NoError(t, err)
	assert.Equal(t, "test body", string(restoredContent))

	// 测试nil情况
	var nilBody io.ReadCloser = nil
	nilBodySlot := &nilBody
	nilBytes, err := readAndRestoreBody(nilBodySlot, -1)
	assert.NoError(t, err)
	assert.Nil(t, nilBytes)
}

func TestMin(t *testing.T) {
	assert.Equal(t, 5, min(5, 10))
	assert.Equal(t, 5, min(10, 5))
	assert.Equal(t, 0, min(0, 5))
	assert.Equal(t, -5, min(-5, 5))
}

// TestSimpleRequestResponseDump tests the formatting of HTTP request and response dumps
func TestSimpleRequestResponseDump(t *testing.T) {
	// 模拟简单的请求/响应转储格式
	var buffer bytes.Buffer
	buffer.WriteString(">>>>>>>>>>>>>>>>>>>>\n")
	buffer.WriteString("POST http://example.com HTTP/1.1\n")
	buffer.WriteString("Content-Type: text/plain\n\n")
	buffer.WriteString("request body\n")
	buffer.WriteString(">>>>>>>>>>>>>>>>>>>>\n")
	buffer.WriteString("<<<<<<<<<<<<<<<<<<<<<<\n")
	buffer.WriteString("HTTP/0.0 200 OK\n")
	buffer.WriteString("Content-Type: application/json\n\n")
	buffer.WriteString(`{"result":"success"}` + "\n")
	buffer.WriteString("<<<<<<<<<<<<<<<<<<<<<<\n")

	output := buffer.String()
	assert.Contains(t, output, "POST http://example.com")
	assert.Contains(t, output, "Content-Type: text/plain")
	assert.Contains(t, output, "request body")
	assert.Contains(t, output, "200 OK")
	assert.Contains(t, output, "Content-Type: application/json")
	assert.Contains(t, output, `{"result":"success"}`)
}
