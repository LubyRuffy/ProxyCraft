package cli

import (
	"bytes"
	"flag" // 修复缺失的导入
	"io"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFlags(t *testing.T) {
	// 测试默认参数
	cfg := ParseFlags()
	assert.NotNil(t, cfg)
	assert.Equal(t, "127.0.0.1", cfg.ListenHost)
	assert.Equal(t, 8080, cfg.ListenPort)

	// 测试自定义参数
	os.Args = []string{"cmd", "-l=192.168.1.1", "-p=9090"}
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError) // 重置flag解析
	cfg = ParseFlags()
	assert.Equal(t, "192.168.1.1", cfg.ListenHost) // 修正之前的错误断言
	assert.Equal(t, "192.168.1.1", cfg.ListenHost) // 修正之前的错误断言
}

// TestPrintHelp tests the PrintHelp function.
func TestPrintHelp(t *testing.T) {
	// 保存原始的os.Stderr并在测试后恢复
	oldStderr := os.Stderr
	defer func() { os.Stderr = oldStderr }()

	// 创建一个新的管道，并将其输出端连接到os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	// 调用PrintHelp函数
	PrintHelp()

	// 关闭写入端并恢复原始的os.Stderr
	w.Close()
	os.Stderr = oldStderr

	// 读取捕获的输出
	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// 验证输出包含帮助信息
	if !strings.Contains(output, "Usage") {
		t.Errorf("Help output should contain 'Usage', but got:\n%s", output)
	}
}
