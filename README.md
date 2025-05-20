# ProxyCraft

[![Go Version](https://img.shields.io/github/go-mod/go-version/LubyRuffy/ProxyCraft)](https://github.com/LubyRuffy/ProxyCraft)
[![License](https://img.shields.io/github/license/LubyRuffy/ProxyCraft)](https://github.com/LubyRuffy/ProxyCraft/blob/main/LICENSE)

ProxyCraft 是一款轻量级、高性能的命令行代理工具，本身为 HTTP 代理协议，支持 HTTPS/HTTP2/SSE 特性，用于进行本地抓包分析。它专为开发人员、安全测试人员和网络管理员设计，提供便捷的流量观察、分析和调试能力。

## 特性

- **核心代理功能**: 提供稳定可靠的 HTTP/HTTPS 代理服务
- **现代协议支持**: 无缝支持 HTTP/1.1、HTTP/2 和 HTTPS (TLS/SSL)
- **SSE 协议支持**: 能够正确代理并展示 Server-Sent Events 流量
- **HTTPS 解密**: 支持中间人 (MITM) 模式以解密和检查 HTTPS 流量
- **HAR 日志记录**: 可将捕获的流量保存为 HAR 格式文件，便于后续分析
- **流量内容输出**: 支持直接在控制台输出请求和响应内容，便于实时分析
- **命令行友好**: 所有功能通过命令行参数和输出进行交互，易于脚本化和集成到自动化流程中
- **轻量高效**: 资源占用低，启动速度快，对系统性能影响小

## 安装

### 从源码编译

```bash
# 克隆仓库
git clone https://github.com/LubyRuffy/ProxyCraft.git
cd ProxyCraft

# 编译
go build -o proxycraft

# 运行
./proxycraft
```

## 使用方法

### 基本用法

```bash
# 启动代理服务器（默认监听 127.0.0.1:8080）
./proxycraft

# 指定监听地址和端口
./proxycraft -l 0.0.0.0 -p 9090

# 启用详细输出
./proxycraft -v

# 将流量保存为 HAR 文件
./proxycraft -o traffic.har

# 在控制台输出流量内容（不显示二进制数据）
./proxycraft -dump

# 导出 CA 证书（用于浏览器信任）
./proxycraft -export-ca proxycraft-ca.pem

# 使用上层代理
./proxycraft -upstream-proxy http://corporate-proxy.example.com:8080
```

### 配置浏览器

1. 启动 ProxyCraft 代理服务器
2. 导出 CA 证书：`./proxycraft -export-ca proxycraft-ca.pem`
3. 将证书导入到浏览器或操作系统的信任存储中
4. 配置浏览器使用代理（默认为 127.0.0.1:8080）

### 使用 curl 测试

```bash
# 使用代理访问 HTTP 网站
curl --proxy http://127.0.0.1:8080 http://example.com

# 使用代理访问 HTTPS 网站（需要 CA 证书）
curl --cacert proxycraft-ca.pem --proxy http://127.0.0.1:8080 https://example.com
```

## 命令行参数

```
-l, -listen-host string   IP address to listen on (default "127.0.0.1")
-p, -listen-port int      Port to listen on (default 8080)
-v, -verbose             Enable verbose output
-o, -output-file string  Save traffic to FILE (HAR format recommended)
-dump                    Dump traffic content to console with headers (binary content will not be displayed)
-filter string           Filter displayed traffic (e.g., "host=example.com")
-export-ca string        Export the root CA certificate to FILEPATH and exit
-use-ca string           Use custom root CA certificate from CERT_PATH
-use-key string          Use custom root CA private key from KEY_PATH
-upstream-proxy string   Upstream proxy URL (e.g., "http://proxy.example.com:8080")
-h, -help                Show this help message and exit
```

## 功能详解

### HTTP/HTTPS 代理

ProxyCraft 作为标准的 HTTP 代理服务器运行，可以处理 HTTP 和 HTTPS 流量。

### HTTP/2 支持

ProxyCraft 能够处理 HTTP/2 流量，包括：

- 通过 ALPN 进行 HTTP/2 协议协商
- 支持 HTTP/2 的帧和流处理
- 在客户端和服务器之间转换 HTTP/1.1 和 HTTP/2（如果必要）

### Server-Sent Events (SSE) 支持

ProxyCraft 能够正确处理 SSE 连接（`Content-Type: text/event-stream`），保持连接持久性，并实时展示接收到的事件数据。

### HAR 日志记录

使用 `-o` 参数可以将捕获的流量保存为 HAR（HTTP Archive）格式文件，包含：

- 完整的请求和响应头部
- 请求和响应正文
- 时间信息
- Cookie 信息
- 其他元数据

这些文件可以被许多工具（如 Chrome DevTools、HAR 查看器等）导入和分析。

### 流量内容输出

使用 `-dump` 参数可以在控制台直接输出捕获的流量内容：

- 显示完整的 HTTP 请求和响应头部
- 自动识别并跳过二进制内容（如图片、视频、PDF 等）
- 显示所有文本格式的请求和响应内容
- 支持 SSE 流式内容的实时输出

输出格式示例：

```
>>>>>>>>>>>>>>>>>>>>
GET http://example.com/api/data HTTP/1.1
User-Agent: Mozilla/5.0
Accept: application/json
Content-Type: application/json

{"query": "test", "limit": 10}
>>>>>>>>>>>>>>>>>>>>
<<<<<<<<<<<<<<<<<<<<
HTTP/1.1 200 OK
Content-Type: application/json
Content-Length: 128
Cache-Control: no-cache

{"status": "success", "data": [...]}
<<<<<<<<<<<<<<<<<<<<
```

对于二进制内容，会显示如下信息：

```
Binary request body detected (1024 bytes), not displaying
```

### CA 证书管理

ProxyCraft 在首次运行时会自动生成自签名根 CA 证书。您可以：

- 使用 `-export-ca` 导出证书以导入到浏览器或系统中
- 使用 `-use-ca` 和 `-use-key` 指定自定义的根 CA 证书和私钥

### 上层代理支持

ProxyCraft 支持通过上层代理转发请求，这在以下场景中非常有用：

- 在需要通过公司代理访问互联网的环境中
- 当您需要通过多个代理链进行访问时
- 当您需要使用特定的出口 IP 进行测试时

使用 `-upstream-proxy` 参数指定上层代理的 URL，例如：

```bash
./proxycraft -upstream-proxy http://proxy.example.com:8080
```

支持的代理协议包括：

- HTTP代理：`http://proxy.example.com:8080`
- HTTPS代理：`https://proxy.example.com:8443`
- SOCKS5代理：`socks5://proxy.example.com:1080`

## 目标用户

- **Web 开发人员**：调试客户端与服务器之间的通信，理解 API 调用，分析 SSE 流
- **API 开发人员**：测试和验证 API 端点的行为和性能
- **安全研究员/渗透测试员**：初步分析应用流量，识别潜在的通信模式
- **网络管理员/DevOps 工程师**：诊断网络连接问题，监控特定应用流量

## 贡献

欢迎贡献代码、报告问题或提出功能建议！请通过 GitHub Issues 或 Pull Requests 参与项目开发。

## 许可证

本项目采用 [MIT 许可证](LICENSE)。

## Web模式

ProxyCraft现在支持Web界面模式，可以在浏览器中查看和分析HTTP/HTTPS流量。

### 启动Web模式

要以Web模式启动ProxyCraft，请使用`-mode web`参数：

```bash
./ProxyCraft -mode web
```

启动后，Web界面默认可在 http://localhost:8081 访问。

### Web界面功能

Web界面提供以下功能：

- 实时显示所有捕获的HTTP/HTTPS请求和响应
- 请求列表支持按方法、主机、路径等字段排序和过滤
- 详细查看请求和响应的头部和内容
- 自动刷新功能，实时显示新捕获的流量
- 支持HTTPS流量的查看
- 支持SSE (Server-Sent Events) 流量的特殊标记

### 界面使用说明

1. 点击请求列表中的任一请求，下方面板会显示该请求的详细信息
2. 在详情面板中可以切换查看请求和响应的详情
3. 点击"刷新"按钮手动刷新请求列表
4. 点击"清空"按钮清除所有已捕获的流量记录
