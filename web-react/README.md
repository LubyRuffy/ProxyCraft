# ProxyCraft React Console

基于 React + Vite + Tailwind CSS + shadcn/ui 的全新 Web 前端骨架。用于逐步取代现有的 Vue 版本。

## 特性概览

- ✅ 已完成基础工程：Vite + React 18 + TypeScript
- ✅ Tailwind CSS 及配套的 `postcss`、`components.json` 配置
- ✅ 预置 `shadcn/ui` 常用工具（`cn` 辅助函数、Button/Badge/Card 组件）
- ✅ 接入 Zustand + React Query，提供统一的 `AppProvider`
- ✅ WebSocket 服务 + `useTrafficStream` Hook，整合实时推送与 HTTP 回退
- ✅ `/traffic` 页面已经使用 store 数据流，可刷新 / 清空 / 选中并查看请求响应详情
- ✅ `RequestResponsePanel` 复刻并排/单列视图、支持拖拽调整宽度与复制 curl
- ✅ WebSocket 自动对接 + SSE 条目轮询刷新，支持断网回退 HTTP 获取详情
- ✅ 采用 `@/` 路径别名，方便逐步迁移模块
- ✅ 默认开启严格的 TypeScript 校验
- ✅ 已搭建 React Router + 布局骨架，`/traffic` 页面提供 UI 占位

## 使用方式

> 需 Node.js ≥ 18。首次使用前请在 `web-react` 目录执行安装：

```bash
npm install
```

常用脚本：

- `npm run dev`：启动开发服务器（默认 5173 端口，自动打开浏览器）。
- `npm run build`：执行 TypeScript 编译并产出生产构建。
- `npm run preview`：本地预览生产构建结果。

## 迁移计划建议

1. **跨项目通信**：根据部署环境配置 `VITE_PROXYCRAFT_SOCKET_URL`，替换 `traffic-service.ts` 中的 mock 数据为真实接口。
2. **状态管理**：完善 Zustand store（分页、筛选、SSE 进度等），视需要补充 React Query 缓存策略。
3. **数据绑定**：将 `/traffic` 页面对接真实流量数据，补完分页、搜索、导出等交互。
4. **组件迁移**：
   - 先迁移 `TrafficList` 列表（表格 + 工具栏 + 分页逻辑）。
   - 再迁移 `RequestResponsePanel` 及其详情组件，使用 Tabs + CodeMirror/Prism 等查看器。
   - 按需封装 shadcn/ui 风格的对话框、表单等常用组件。
5. **主题与样式**：Tailwind 变量已与 shadcn/ui 对齐，可按需扩展；如需暗色模式，可在 `App.tsx` 中引入主题切换逻辑。
6. **构建集成**：待主要页面迁移完成后，再调整顶层 `build_web.sh` 等脚本，确保 CI/CD 可以在新前端上运行。
7. **DevTools**：按需启用 React Query / Zustand DevTools 以辅助调试。

> 建议迁移过程中保持 Vue 版本可用，待 React 端覆盖关键功能后再统一替换。

## 下一步

- [x] 在 `src` 下创建 `routes` 与 `layouts` 目录，搭建基础路由框架。
- [ ] 配置 `VITE_PROXYCRAFT_SOCKET_URL` 并对接真实的 Socket.IO 服务端。
- [ ] 替换 mock `traffic-service.ts`，与 Go API 保持一致。
- [ ] 扩展 `/traffic` 表格的筛选 / 分页 / SSE 进度展示。
- [ ] 编写端到端测试（可选：Playwright）以覆盖关键交互。

欢迎继续补充迁移步骤或提出新的组件需求。
