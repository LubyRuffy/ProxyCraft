import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useCallback, useEffect, useMemo } from 'react';
import { RequestResponsePanel } from '@/components/request-response-panel';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { useTrafficStream } from '@/hooks/use-traffic-stream';
import { cn } from '@/lib/utils';
import { useTrafficStore } from '@/stores/use-traffic-store';
const formatBytes = (bytes) => {
    if (bytes === 0)
        return '0 B';
    const units = ['B', 'KB', 'MB', 'GB'];
    const power = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
    return `${(bytes / 1024 ** power).toFixed(power === 0 ? 0 : 1)} ${units[power]}`;
};
const statusTint = (status) => {
    if (status >= 500)
        return 'text-red-500';
    if (status >= 400)
        return 'text-amber-500';
    if (status >= 200)
        return 'text-emerald-500';
    return 'text-muted-foreground';
};
export function TrafficPage() {
    const entries = useTrafficStore((state) => state.entries);
    const selectedId = useTrafficStore((state) => state.selectedId);
    const selectEntry = useTrafficStore((state) => state.selectEntry);
    const detail = useTrafficStore((state) => state.detail);
    const loading = useTrafficStore((state) => state.loading);
    const error = useTrafficStore((state) => state.error);
    const connected = useTrafficStore((state) => state.connected);
    const transport = useTrafficStore((state) => state.transport);
    const { refresh, reconnect, clearRemoteTraffic } = useTrafficStream();
    useEffect(() => {
        if (entries.length === 0) {
            refresh();
        }
    }, [entries.length, refresh]);
    const list = useMemo(() => entries ?? [], [entries]);
    const selectedEntry = useMemo(() => list.find((item) => item.id === selectedId) ?? null, [list, selectedId]);
    const lastUpdated = useMemo(() => {
        if (!list.length)
            return '未同步';
        const timestamps = list
            .map((entry) => entry.endTime || entry.startTime)
            .filter(Boolean)
            .map((value) => new Date(value).getTime());
        if (!timestamps.length)
            return '未知';
        return new Date(Math.max(...timestamps)).toLocaleTimeString();
    }, [list]);
    const handleClear = useCallback(async () => {
        const confirmed = window.confirm('确定要清空所有流量数据吗？此操作不可恢复。');
        if (!confirmed) {
            return;
        }
        await clearRemoteTraffic();
    }, [clearRemoteTraffic]);
    const handleRefresh = useCallback(() => {
        refresh();
    }, [refresh]);
    const handleReconnect = useCallback(() => {
        reconnect();
    }, [reconnect]);
    return (_jsxs("section", { className: "flex h-full flex-1 flex-col gap-4 px-6 py-4", children: [_jsxs("div", { className: "flex items-center justify-between gap-4 border-b pb-3", children: [_jsxs("div", { className: "flex items-center gap-4", children: [_jsx("h1", { className: "text-lg font-semibold", children: "\u6D41\u91CF\u5217\u8868" }), _jsxs("div", { className: "flex gap-2", children: [_jsx(Button, { size: "sm", onClick: handleRefresh, disabled: loading, children: loading ? '处理中…' : '刷新' }), _jsx(Button, { size: "sm", variant: "destructive", onClick: handleClear, disabled: loading, children: "\u6E05\u7A7A" }), _jsx(Button, { size: "sm", variant: "secondary", onClick: handleReconnect, disabled: loading, children: "\u91CD\u65B0\u8FDE\u63A5" })] })] }), _jsxs("div", { className: "flex items-center gap-3 text-sm text-muted-foreground", children: [_jsx(Badge, { variant: connected ? 'success' : 'warning', children: connected ? 'WebSocket 已连接' : 'WebSocket 未连接' }), _jsxs("span", { children: ["\u6700\u540E\u66F4\u65B0\uFF1A", lastUpdated] })] })] }), error ? (_jsx("div", { className: "rounded-lg border border-destructive/50 bg-destructive/5 p-4 text-sm text-destructive-foreground", children: error })) : null, _jsx("div", { className: "flex-1 overflow-hidden", children: _jsxs("div", { className: "h-full overflow-auto rounded-lg border", children: [_jsxs("table", { className: "min-w-full text-sm", children: [_jsx("thead", { className: "bg-muted/50 text-xs uppercase text-muted-foreground sticky top-0", children: _jsxs("tr", { children: [_jsx("th", { className: "px-4 py-2 text-left font-medium w-16", children: "\u65B9\u6CD5" }), _jsx("th", { className: "px-4 py-2 text-left font-medium min-w-32", children: "Host" }), _jsx("th", { className: "px-4 py-2 text-left font-medium", children: "Path" }), _jsx("th", { className: "px-4 py-2 text-left font-medium w-16", children: "Code" }), _jsx("th", { className: "px-4 py-2 text-left font-medium w-24", children: "MIME" }), _jsx("th", { className: "px-4 py-2 text-left font-medium w-20", children: "Size" }), _jsx("th", { className: "px-4 py-2 text-left font-medium w-20", children: "Cost" }), _jsx("th", { className: "px-4 py-2 text-left font-medium w-24", children: "Tags" })] }) }), _jsx("tbody", { children: list.map((entry) => (_jsxs("tr", { onClick: () => selectEntry(entry.id), className: cn('cursor-pointer transition-colors hover:bg-muted/70', selectedId === entry.id && 'bg-secondary/40'), children: [_jsx("td", { className: "px-4 py-2 font-medium", children: entry.method }), _jsx("td", { className: "px-4 py-2 truncate text-muted-foreground", children: entry.host }), _jsx("td", { className: "px-4 py-2 truncate", children: entry.path }), _jsx("td", { className: cn('px-4 py-2 font-mono', statusTint(entry.statusCode)), children: entry.statusCode }), _jsx("td", { className: "px-4 py-2 truncate text-muted-foreground", children: entry.contentType }), _jsx("td", { className: "px-4 py-2 text-muted-foreground", children: formatBytes(entry.contentSize) }), _jsxs("td", { className: "px-4 py-2 text-muted-foreground", children: [entry.duration, " ms"] }), _jsx("td", { className: "px-4 py-2", children: _jsxs("div", { className: "flex gap-1", children: [entry.isHTTPS ? _jsx(Badge, { variant: "success", children: "HTTPS" }) : null, entry.isSSE ? _jsx(Badge, { variant: "warning", children: "SSE" }) : null] }) })] }, entry.id))) })] }), list.length === 0 ? (_jsx("div", { className: "flex items-center justify-center p-20 text-sm text-muted-foreground", children: loading ? '加载中…' : '暂无流量记录' })) : null] }) }), _jsx("div", { className: "h-80 border rounded-lg overflow-hidden", children: _jsxs("div", { className: "flex h-full flex-col", children: [_jsxs("div", { className: "flex items-center justify-between border-b px-4 py-2", children: [_jsx("h2", { className: "text-base font-semibold", children: "\u8BF7\u6C42 / \u54CD\u5E94\u8BE6\u60C5" }), selectedId && (_jsxs("div", { className: "flex items-center gap-2 text-sm", children: [_jsxs(Badge, { variant: "outline", children: ["ID: ", selectedId] }), loading ? _jsx(Badge, { variant: "warning", children: "\u52A0\u8F7D\u4E2D\u2026" }) : null] }))] }), _jsx("div", { className: "flex-1 p-4", children: selectedId ? (_jsx(RequestResponsePanel, { entry: selectedEntry ?? undefined, detail: detail, loading: loading })) : (_jsx("div", { className: "flex h-full items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground", children: "\u8BF7\u9009\u62E9\u5217\u8868\u4E2D\u7684\u4E00\u6761\u6D41\u91CF\u8BB0\u5F55\u4EE5\u67E5\u770B\u8BE6\u60C5\u3002" })) })] }) })] }));
}
