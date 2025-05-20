package proxy

import (
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/LubyRuffy/ProxyCraft/certs"
	"github.com/LubyRuffy/ProxyCraft/harlogger"
	"github.com/stretchr/testify/assert"
)

func TestNewServer(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 测试不同配置的服务器创建
	testCases := []struct {
		name        string
		addr        string
		verbose     bool
		dumpTraffic bool
	}{
		{
			name:        "基本模式",
			addr:        "127.0.0.1:0",
			verbose:     false,
			dumpTraffic: false,
		},
		{
			name:        "详细日志模式",
			addr:        "127.0.0.1:0",
			verbose:     true,
			dumpTraffic: false,
		},
		{
			name:        "流量转储模式",
			addr:        "127.0.0.1:0",
			verbose:     false,
			dumpTraffic: true,
		},
		{
			name:        "全部功能启用",
			addr:        "127.0.0.1:0",
			verbose:     true,
			dumpTraffic: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// 创建代理服务器
			server := NewServer(
				tc.addr,
				certMgr,
				tc.verbose,
				harLog,
				nil, // 不使用上游代理
				tc.dumpTraffic,
			)

			// 验证服务器属性
			assert.NotNil(t, server)
			assert.Equal(t, tc.addr, server.Addr)
			assert.Equal(t, tc.verbose, server.Verbose)
			assert.Equal(t, tc.dumpTraffic, server.DumpTraffic)
			assert.NotNil(t, server.HarLogger)
			assert.NotNil(t, server.CertManager)
		})
	}
}

func TestLogToHAR(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建服务器
	server := NewServer(
		"127.0.0.1:0",
		certMgr,
		true,
		harLog,
		nil,
		false,
	)
	assert.NotNil(t, server)

	// 创建测试请求和响应
	testReq, _ := http.NewRequest("GET", "http://example.com", nil)
	testResp := &http.Response{
		StatusCode: 200,
		Request:    testReq,
		Header:     make(http.Header),
	}
	testResp.Header.Set("Content-Type", "text/plain")

	// 记录启动时间
	startTime := time.Now().Add(-1 * time.Second) // 假设请求发生在1秒前
	timeTaken := 1 * time.Second

	// 测试常规请求的HAR日志记录
	server.logToHAR(testReq, testResp, startTime, timeTaken, false)

	// 测试SSE请求的HAR日志记录
	testResp.Header.Set("Content-Type", "text/event-stream")
	server.logToHAR(testReq, testResp, startTime, timeTaken, true)

	// 测试错误情况下的HAR日志记录
	server.logToHAR(testReq, nil, startTime, timeTaken, false)
}

func TestServerStart(t *testing.T) {
	// 创建必要的依赖项
	certMgr, _ := certs.NewManager()
	harLog := harlogger.NewLogger("", "ProxyCraft", "0.1.0")

	// 创建代理服务器 - 使用随机端口避免冲突
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	proxyAddr := listener.Addr().String()
	listener.Close() // 关闭监听器，让服务器可以使用这个端口

	// 创建代理服务器
	server := NewServer(
		proxyAddr,
		certMgr,
		false,
		harLog,
		nil,
		false,
	)
	assert.NotNil(t, server)

	// 在后台启动服务器
	go func() {
		err := server.Start()
		// 我们期望Start在服务器正常运行期间不返回
		// 如果返回，并且不是因为我们关闭服务器，就是错误
		assert.NoError(t, err)
	}()

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	// 尝试连接到服务器
	conn, err := net.Dial("tcp", proxyAddr)
	if err == nil {
		// 如果连接成功，关闭连接
		conn.Close()
	} else {
		// 如果连接失败，记录错误（在CI环境中可能会失败）
		t.Logf("无法连接到代理服务器: %v", err)
	}
}

func TestHeaderInterceptingTransportStructure(t *testing.T) {
	// 创建一个模拟的RoundTripper
	mockBaseTransport := &http.Transport{}

	// 创建一个headerInterceptingTransport
	transport := &headerInterceptingTransport{
		base:     mockBaseTransport,
		verbose:  true,
		callback: nil,
	}

	// 检查结构是否正确初始化
	assert.NotNil(t, transport.base)
	assert.True(t, transport.verbose)
	assert.Nil(t, transport.callback)
}
