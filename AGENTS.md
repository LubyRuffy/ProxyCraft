# Agent Rules

## Frontend Change Protocol
- Always run `npm run build` in `web-react/` after any frontend change or dependency update.
- Do not report completion until the build passes with exit code 0.
- Avoid hardcoded color utility classes (e.g. `text-red-500`, `bg-amber-400`). Use theme token classes only (e.g. `text-primary`, `bg-accent`).
- Keep Tailwind v4 usage consistent with `@import "tailwindcss";` in `web-react/src/index.css`.
- Do not commit generated `.js` files under `web-react/src/`. Only `.ts/.tsx` source files should live there. If such files appear, delete them and ensure TypeScript is not emitting into `src/`.
- Add shadcn components via `npx shadcn@latest add [component]` instead of hand-writing them.

## Scope Safety
- Operate only within this repository.
- Do not search or modify paths outside the repo root.

## Proxy/SSE定位速查
- WebSocket推送与前端订阅：`api/websocket.go`（事件名）↔ `web-react/src/services/websocket.ts`、`web-react/src/hooks/use-traffic-stream.ts`。
- 流量条目结构与状态：`proxy/handlers/web_handler.go`（TrafficEntry、OnRequest/OnResponse/OnSSE、完成标志）。
- SSE处理与转发：`proxy/sse_handler.go`（handleSSE、SSE事件分段/flush、isServerSentEvent）。
- 代理请求链路：`proxy/roundtrip.go`（prepare/send/process，SSE判定与超时配置）。
- HTTP/HTTPS入口：`proxy/http_handler.go`、`proxy/https_handler.go`（SSE分支与TLS流式转发）。
