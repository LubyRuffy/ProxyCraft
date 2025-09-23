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
- **实时通信**: 采用 Socket.IO v4 实现前后端实时通信，支持 WebSocket 和 HTTP 轮询双通道

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

### 实时通信 (Socket.IO)

ProxyCraft 使用 Socket.IO v3 实现前后端实时通信：

- **后端**: 采用 `github.com/zishang520/socket.io/servers/socket/v3` v3.0.0-rc.5 实现 WebSocket 服务器
- **前端**: 使用 `socket.io-client` v4.8.1 实现 WebSocket 客户端
- **协议支持**: 支持 WebSocket 和 HTTP 轮询双通道，确保在各种网络环境下都能正常工作
- **自动重连**: 内置重连机制，网络恢复后自动重新连接
- **事件驱动**: 基于事件的消息推送，实时更新流量数据

**WebSocket 事件：**
- `traffic_entries` - 获取所有流量条目列表
- `traffic_new_entry` - 新的流量条目推送
- `request_details` - 获取请求详情
- `response_details` - 获取响应详情
- `traffic_clear` - 清空所有流量条目

**故障排除：**
如果Web界面显示"处理中"或"加载中"无法加载请求/响应详情，请：

1. **检查WebSocket连接状态**
   ```bash
   # 确保后端服务器运行在8081端口
   curl http://localhost:8081/health
   curl http://localhost:8081/socket.io/
   ```

2. **查看浏览器开发者工具**
   - Network标签：查看WebSocket消息是否正确发送和接收
   - Console标签：查看JavaScript错误信息

3. **检查服务器日志**
   ```bash
   # 启动Web模式并查看日志（包含详细的事件处理日志）
   ./ProxyCraft -mode web -v
   ```

4. **常见问题**
   - 确保前端已正确构建：`./build_web.sh`
   - 检查端口占用：`lsof -i :8081`
   - 清除浏览器缓存和Cookie
   - 尝试不同的浏览器测试

5. **调试模式**
   - 在浏览器中打开开发者工具
   - 查看WebSocket消息的发送和接收
   - 检查请求参数格式是否正确
   - 查看服务器日志中的"接收到获取...请求"消息确认事件处理是否被调用

## 🔧 已修复的问题

### Socket.IO v3 事件处理架构修正

**问题**：WebSocket事件处理函数没有被调用，点击请求详情时一直显示"加载中"

**根本原因**：
1. 事件监听器位置错误 - 业务事件应该在`client.On()`中监听，而不是`Server.On()`
2. 缺少正确的事件处理器初始化
3. 对Socket.IO v3的事件处理架构理解有误

**修复内容**：
1. ✅ **正确的事件处理架构**
   ```
   Socket.IO v3 正确的事件处理架构：

   前端发送事件 → client.On("event") → 后端处理 → client.Emit("event") → 前端接收

   具体流程：
   1. 前端：socket.emit('traffic_entries')
   2. 后端：client.On('traffic_entries', handler)
   3. 后端处理完成后：client.Emit('traffic_entries', data)
   4. 前端：socket.on('traffic_entries', handler)
   ```

2. ✅ **正确的事件处理器初始化**
   ```go
   // 在NewServer中正确设置事件处理器
   wsServer, err := NewWebSocketServer(webHandler)
   if err != nil {
       log.Printf("Warning: Could not initialize WebSocket server: %v", err)
   } else {
       server.WebSocketServer = wsServer
       // 设置WebSocket事件处理器
       server.WebSocketServer.setupEventHandlers()
   }
   ```

3. ✅ **完整的事件处理链**
   ```go
   ws.Server.On("connection", func(clients ...interface{}) {
       client := clients[0].(*socket.Socket)

       // 客户端特定事件（连接内）
       client.On("disconnect", handler)
       client.On("error", handler)

       // 业务事件（连接内）
       client.On(EventTrafficEntries, handler)
       client.On(EventRequestDetails, handler)
       client.On(EventResponseDetails, handler)
       client.On(EventTrafficClear, handler)
       client.On("ping", handler)
       client.On("heartbeat", handler)
   })
   ```

4. ✅ **Socket.IO v3 API结构**
   ```
   Server.On("connection") → 处理客户端连接
   └── client.On("event_name") → 处理客户端发送的所有事件
       ├── 业务事件: traffic_entries, request_details, etc.
       ├── 系统事件: ping, heartbeat, etc.
       └── 错误事件: error, disconnect, etc.
   ```

5. ✅ **详细的日志记录**
   - 添加了事件接收的日志输出
   - 添加了数据处理的详细日志
   - 添加了错误处理的日志

**验证方法**：
- 在服务器日志中查看"接收到获取请求详情请求"消息
- 在浏览器开发者工具中查看WebSocket消息的发送和接收
- 确认不再显示"加载中"状态
- 检查服务器日志中是否出现"WebSocket服务器已启动"消息

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

自 vNext 起，Web 控制台代码迁移至 React + Tailwind（目录 `web-react/`）。旧的 Vue 版本保留在 `web/` 目录中，作为回退参考，但官方构建和发布流程已经默认使用 React 版本。

### 启动Web模式

要以Web模式启动ProxyCraft，请使用`-mode web`参数：

```bash
./ProxyCraft -mode web
```

启动后，Web界面默认可在 http://localhost:8081 访问。

### Web界面功能

Web界面提供以下功能：

- 实时显示所有捕获的HTTP/HTTPS请求和响应
- 请求列表支持按方法、主机、路径等字段排序和筛选（后续逐步增强）
- 详细查看请求和响应的头部与正文，支持复制为 `curl`
- Socket.IO v4 实时推送与 HTTP 回退双通道，SSE 流量自动补拉最新数据
- 支持HTTPS流量的查看
- 支持SSE (Server-Sent Events) 流量的特殊标记

### 界面使用说明

1. 点击请求列表中的任一请求，下方面板会显示该请求的详细信息
2. 在详情面板中可以切换查看请求和响应的详情
3. 点击"刷新"按钮手动刷新请求列表，或等待实时推送
4. 点击"清空"按钮清除所有已捕获的流量记录（需确认）
5. 在详情面板中可拖拽调整请求/响应视图宽度，并复制请求为 `curl`

### 开发/构建 React Web 控制台

React 版前端位于 `web-react/` 目录。常用操作如下：

```bash
cd web-react
# 推荐使用 cnpm（如果已安装），否则使用 npm
cnpm install  # 或 npm install

# 本地开发
cnpm run dev  # 或 npm run dev

# 生产构建（可通过环境变量指定 WebSocket 地址）
VITE_PROXYCRAFT_SOCKET_URL=http://localhost:8081 cnpm run build
```

构建完成的前端产物会输出至 `api/dist/` 目录（由 Vite 配置和 `build_web.sh` 脚本负责），供服务端在 Web 模式下静态托管。

顶层脚本 `./build_web.sh` 已改为默认使用 React 版本：

```bash
./build_web.sh                 # 默认假定 WebSocket 在 http://localhost:8081
VITE_PROXYCRAFT_SOCKET_URL=https://proxycraft.example.com ./build_web.sh
```

如需回滚或参考旧版实现，可进入 `web/` 目录查看 Vue 代码。
