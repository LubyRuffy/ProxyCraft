package cli

import (
	"flag" // 修复缺失的导入
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
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
