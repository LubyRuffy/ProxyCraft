import { jsx as _jsx, jsxs as _jsxs } from "react/jsx-runtime";
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
const DISPLAY_OPTIONS = [
    { key: 'split', label: '并排显示' },
    { key: 'request', label: '仅请求' },
    { key: 'response', label: '仅响应' },
];
const formatHeaders = (message) => {
    if (!message?.headers)
        return '{}';
    return JSON.stringify(message.headers, null, 2);
};
const formatBody = (body) => {
    if (body === undefined || body === null || body === '')
        return '无正文';
    if (typeof body === 'string') {
        return body;
    }
    try {
        return JSON.stringify(body, null, 2);
    }
    catch (error) {
        return String(body);
    }
};
const buildUrl = (entry) => {
    if (entry.url) {
        return entry.url;
    }
    const prefix = entry.isHTTPS ? 'https://' : 'http://';
    return `${prefix}${entry.host || ''}${entry.path || ''}`;
};
const buildCurlCommand = (entry, detail) => {
    if (!entry || !detail?.request)
        return '';
    const { method } = entry;
    const request = detail.request;
    let command = `curl -X ${method}`;
    const url = buildUrl(entry);
    command += ` "${url}"`;
    const headers = request.headers ?? {};
    Object.entries(headers).forEach(([key, value]) => {
        if (value === undefined || value === null || value === '') {
            return;
        }
        command += ` -H "${key}: ${value}"`;
    });
    const body = request.body;
    if (!body || body === '<Binary data, 0 bytes>') {
        return command;
    }
    let bodyString = '';
    if (typeof body === 'string') {
        bodyString = body;
    }
    else {
        try {
            bodyString = JSON.stringify(body);
        }
        catch (error) {
            bodyString = String(body);
        }
    }
    if (bodyString) {
        command += ` --data '${bodyString}'`;
    }
    return command;
};
export function RequestResponsePanel({ entry, detail, loading }) {
    const [mode, setMode] = useState('split');
    const [copyState, setCopyState] = useState('idle');
    const [requestRatio, setRequestRatio] = useState(0.5);
    const containerRef = useRef(null);
    const isResizing = useRef(false);
    const startX = useRef(0);
    const startRatio = useRef(0.5);
    useEffect(() => {
        setCopyState('idle');
    }, [entry?.id]);
    const handleCopy = useCallback(async () => {
        const command = buildCurlCommand(entry, detail);
        if (!command) {
            setCopyState('error');
            return;
        }
        try {
            await navigator.clipboard.writeText(command);
            setCopyState('success');
            setTimeout(() => setCopyState('idle'), 2000);
        }
        catch (error) {
            console.error('复制请求为 curl 时失败', error);
            setCopyState('error');
            setTimeout(() => setCopyState('idle'), 2000);
        }
    }, [detail, entry]);
    const onMouseMove = useCallback((event) => {
        if (!isResizing.current || !containerRef.current)
            return;
        const { width } = containerRef.current.getBoundingClientRect();
        if (!width)
            return;
        const delta = event.clientX - startX.current;
        const nextRatio = Math.min(0.8, Math.max(0.2, startRatio.current + delta / width));
        setRequestRatio(nextRatio);
    }, []);
    const stopResize = useCallback(() => {
        if (!isResizing.current)
            return;
        isResizing.current = false;
        document.removeEventListener('mousemove', onMouseMove);
        document.removeEventListener('mouseup', stopResize);
        document.body.style.userSelect = '';
    }, [onMouseMove]);
    const startResize = useCallback((event) => {
        if (mode !== 'split')
            return;
        isResizing.current = true;
        startX.current = event.clientX;
        startRatio.current = requestRatio;
        document.addEventListener('mousemove', onMouseMove);
        document.addEventListener('mouseup', stopResize);
        document.body.style.userSelect = 'none';
    }, [mode, onMouseMove, requestRatio, stopResize]);
    useEffect(() => {
        return () => {
            stopResize();
        };
    }, [stopResize]);
    const hasRequest = Boolean(detail?.request);
    const hasResponse = Boolean(detail?.response);
    const content = useMemo(() => {
        if (!entry || (!hasRequest && !hasResponse)) {
            return (_jsx("div", { className: "flex h-full items-center justify-center rounded-lg border border-dashed p-8 text-sm text-muted-foreground", children: "\u8BF7\u9009\u62E9\u4E00\u6761\u6D41\u91CF\u8BB0\u5F55\u4EE5\u67E5\u770B\u8BE6\u60C5\u3002" }));
        }
        const showRequest = mode === 'split' || mode === 'request';
        const showResponse = mode === 'split' || mode === 'response';
        return (_jsxs("div", { ref: containerRef, className: cn('flex h-full w-full gap-3', mode !== 'split' && 'gap-0'), style: { minHeight: 320 }, children: [showRequest ? (_jsxs("section", { className: "flex flex-col rounded-lg border bg-card", style: { flex: mode === 'split' ? requestRatio : 1 }, children: [_jsx("header", { className: "border-b px-4 py-2 text-sm font-semibold", children: "\u8BF7\u6C42" }), _jsxs("div", { className: "flex-1 space-y-3 overflow-auto p-4", children: [_jsxs("div", { children: [_jsx("p", { className: "text-xs font-medium text-muted-foreground", children: "\u8BF7\u6C42\u5934" }), _jsx("pre", { className: "mt-1 max-h-48 overflow-auto rounded bg-muted/60 p-3 text-xs", children: formatHeaders(detail?.request) })] }), _jsxs("div", { children: [_jsx("p", { className: "text-xs font-medium text-muted-foreground", children: "\u8BF7\u6C42\u4F53" }), _jsx("pre", { className: "mt-1 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-muted/60 p-3 text-xs", children: formatBody(detail?.request?.body) })] })] })] })) : null, mode === 'split' ? (_jsx("div", { role: "separator", "aria-orientation": "vertical", className: "flex w-1 cursor-col-resize items-stretch", onMouseDown: startResize, children: _jsx("span", { className: "mx-auto h-full w-px bg-border" }) })) : null, showResponse ? (_jsxs("section", { className: "flex flex-col rounded-lg border bg-card", style: { flex: mode === 'split' ? 1 - requestRatio : 1 }, children: [_jsx("header", { className: "border-b px-4 py-2 text-sm font-semibold", children: "\u54CD\u5E94" }), _jsxs("div", { className: "flex-1 space-y-3 overflow-auto p-4", children: [_jsxs("div", { children: [_jsx("p", { className: "text-xs font-medium text-muted-foreground", children: "\u54CD\u5E94\u5934" }), _jsx("pre", { className: "mt-1 max-h-48 overflow-auto rounded bg-muted/60 p-3 text-xs", children: formatHeaders(detail?.response) })] }), _jsxs("div", { children: [_jsx("p", { className: "text-xs font-medium text-muted-foreground", children: "\u54CD\u5E94\u4F53" }), _jsx("pre", { className: "mt-1 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-muted/60 p-3 text-xs", children: formatBody(detail?.response?.body) })] })] })] })) : null] }));
    }, [detail?.request, detail?.response, entry, hasRequest, hasResponse, mode, requestRatio, startResize]);
    return (_jsxs("div", { className: "flex h-full flex-col gap-3", children: [_jsxs("div", { className: "flex flex-wrap items-center justify-between gap-3", children: [_jsx("div", { className: "flex items-center gap-1", children: DISPLAY_OPTIONS.map((option) => (_jsx(Button, { size: "sm", variant: mode === option.key ? 'secondary' : 'outline', className: cn('rounded-none first:rounded-l-md last:rounded-r-md'), onClick: () => setMode(option.key), children: option.label }, option.key))) }), _jsxs("div", { className: "flex items-center gap-2", children: [copyState === 'success' ? _jsx(Badge, { variant: "success", children: "\u5DF2\u590D\u5236" }) : null, copyState === 'error' ? _jsx(Badge, { variant: "destructive", children: "\u590D\u5236\u5931\u8D25" }) : null, _jsx(Button, { size: "sm", variant: "outline", onClick: handleCopy, disabled: loading, children: "\u590D\u5236\u4E3A curl" })] })] }), _jsx("div", { className: cn('relative flex-1 rounded-lg border p-4', loading && 'opacity-70'), children: content })] }));
}
