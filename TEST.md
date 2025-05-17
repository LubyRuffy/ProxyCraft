# 测试用例的构造

## 支持协议
- 简单的http请求可以响应：`curl -x http://127.0.0.1:8080 -v http://ip.bmh.im`
- 简单的https请求可以响应：`curl -x http://127.0.0.1:8080 -v https://ip.bmh.im`
- 支持http2：能够正常返回。`curl -x http://127.0.0.1:8080 -v --http2 https://ip.bmh.im`
- 支持sse：能够看到流失输出而不是一次性输出。`curl -x http://127.0.0.1:8080 http://127.0.0.1:1234/v1/chat/completions -H "Content-Type: application/json" -d '{"model": "qwen3-4b","stream": true,"messages": [{"role": "user","content": "天空为什么是蓝色的？/no_think"}]}'`

## 支持抓包

## 支持上层代理
- 上层代理才能访问的网站：`curl -v -x http://127.0.0.1:8080 https://www.google.com`