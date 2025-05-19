package main

import (
	"bytes"
	"flag"
	"io"
	"log"
	"os"
	"testing"

	"github.com/LubyRuffy/ProxyCraft/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestMainFunctionality tests the main package functionality
func TestMainFunctionality(t *testing.T) {
	// 保存原始的os.Args并在测试后恢复
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	// 保存原始标准输出并在测试后恢复
	oldOutput := os.Stdout
	defer func() { os.Stdout = oldOutput }()

	// 保存原始的flag.CommandLine并在测试后恢复
	origFlagCommandLine := flag.CommandLine
	defer func() { flag.CommandLine = origFlagCommandLine }()

	// 保存原始的logger输出并在测试后恢复
	origLogOutput := log.Writer()
	defer func() { log.SetOutput(origLogOutput) }()

	t.Run("export_ca_flag", func(t *testing.T) {
		// 重置flag以便使用-export-ca标志
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)

		// 创建一个临时文件来导出CA证书
		tmpCAFile, err := os.CreateTemp("", "test-ca-*.pem")
		require.NoError(t, err)
		tmpCAPath := tmpCAFile.Name()
		tmpCAFile.Close()
		defer os.Remove(tmpCAPath)

		os.Args = []string{"cmd", "-export-ca", tmpCAPath}

		// 重定向标准输出以捕获输出
		r, w, _ := os.Pipe()
		os.Stdout = w
		log.SetOutput(w)

		// 解析命令行参数
		cfg := cli.ParseFlags()
		assert.Equal(t, tmpCAPath, cfg.ExportCAPath)

		// 关闭管道写入端
		w.Close()

		// 读取捕获的输出，但不验证内容
		var buf bytes.Buffer
		io.Copy(&buf, r)

		// 验证flag被正确解析
		assert.True(t, len(cfg.ExportCAPath) > 0, "ExportCAPath应该被设置")
	})

	t.Run("custom_listen_address", func(t *testing.T) {
		// 重置flag以便设置自定义监听地址
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		os.Args = []string{"cmd", "-listen-host", "127.0.0.1", "-listen-port", "9090"}

		// 解析命令行参数
		cfg := cli.ParseFlags()

		// 验证监听地址被正确设置
		assert.Equal(t, "127.0.0.1", cfg.ListenHost)
		assert.Equal(t, 9090, cfg.ListenPort)
	})

	t.Run("har_logging_options", func(t *testing.T) {
		// 重置flag以便测试HAR日志选项
		flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
		os.Args = []string{"cmd", "-o", "test.har", "-auto-save", "60"}

		// 解析命令行参数
		cfg := cli.ParseFlags()

		// 验证HAR日志选项被正确设置
		assert.Equal(t, "test.har", cfg.OutputFile)
		assert.Equal(t, 60, cfg.AutoSaveInterval)
	})
}
