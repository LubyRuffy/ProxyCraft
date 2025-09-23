import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import type { MouseEvent as ReactMouseEvent } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { HttpMessage, TrafficDetail, TrafficEntry } from '@/types/traffic';

const DISPLAY_OPTIONS = [
  { key: 'split', label: '并排显示' },
  { key: 'request', label: '仅请求' },
  { key: 'response', label: '仅响应' },
] as const;

type DisplayMode = (typeof DISPLAY_OPTIONS)[number]['key'];

type CopyState = 'idle' | 'success' | 'error';

type RequestResponsePanelProps = {
  entry?: TrafficEntry | null;
  detail?: TrafficDetail;
  loading?: boolean;
};

const formatHeaders = (message?: HttpMessage) => {
  if (!message?.headers) return '{}';
  return JSON.stringify(message.headers, null, 2);
};

const formatBody = (body?: unknown) => {
  if (body === undefined || body === null || body === '') return '无正文';
  if (typeof body === 'string') {
    return body;
  }
  try {
    return JSON.stringify(body, null, 2);
  } catch (error) {
    return String(body);
  }
};

const buildUrl = (entry: TrafficEntry) => {
  if (entry.url) {
    return entry.url;
  }
  const prefix = entry.isHTTPS ? 'https://' : 'http://';
  return `${prefix}${entry.host || ''}${entry.path || ''}`;
};

const buildCurlCommand = (entry?: TrafficEntry | null, detail?: TrafficDetail) => {
  if (!entry || !detail?.request) return '';
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
  } else {
    try {
      bodyString = JSON.stringify(body);
    } catch (error) {
      bodyString = String(body);
    }
  }

  if (bodyString) {
    command += ` --data '${bodyString}'`;
  }

  return command;
};

export function RequestResponsePanel({ entry, detail, loading }: RequestResponsePanelProps) {
  const [mode, setMode] = useState<DisplayMode>('split');
  const [copyState, setCopyState] = useState<CopyState>('idle');
  const [requestRatio, setRequestRatio] = useState(0.5);
  const containerRef = useRef<HTMLDivElement | null>(null);
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
    } catch (error) {
      console.error('复制请求为 curl 时失败', error);
      setCopyState('error');
      setTimeout(() => setCopyState('idle'), 2000);
    }
  }, [detail, entry]);

  const onMouseMove = useCallback(
    (event: MouseEvent) => {
      if (!isResizing.current || !containerRef.current) return;
      const { width } = containerRef.current.getBoundingClientRect();
      if (!width) return;
      const delta = event.clientX - startX.current;
      const nextRatio = Math.min(0.8, Math.max(0.2, startRatio.current + delta / width));
      setRequestRatio(nextRatio);
    },
    []
  );

  const stopResize = useCallback(() => {
    if (!isResizing.current) return;
    isResizing.current = false;
    document.removeEventListener('mousemove', onMouseMove);
    document.removeEventListener('mouseup', stopResize);
    document.body.style.userSelect = '';
  }, [onMouseMove]);

  const startResize = useCallback(
    (event: ReactMouseEvent<HTMLDivElement>) => {
      if (mode !== 'split') return;
      isResizing.current = true;
      startX.current = event.clientX;
      startRatio.current = requestRatio;
      document.addEventListener('mousemove', onMouseMove);
      document.addEventListener('mouseup', stopResize);
      document.body.style.userSelect = 'none';
    },
    [mode, onMouseMove, requestRatio, stopResize]
  );

  useEffect(() => {
    return () => {
      stopResize();
    };
  }, [stopResize]);

  const hasRequest = Boolean(detail?.request);
  const hasResponse = Boolean(detail?.response);

  const content = useMemo(() => {
    if (!entry || (!hasRequest && !hasResponse)) {
      return (
        <div className="flex h-full items-center justify-center rounded-lg border border-dashed p-8 text-sm text-muted-foreground">
          请选择一条流量记录以查看详情。
        </div>
      );
    }

    const showRequest = mode === 'split' || mode === 'request';
    const showResponse = mode === 'split' || mode === 'response';

    return (
      <div
        ref={containerRef}
        className={cn('flex h-full w-full gap-3', mode !== 'split' && 'gap-0')}
        style={{ minHeight: 320 }}
      >
        {showRequest ? (
          <section className="flex flex-col rounded-lg border bg-card" style={{ flex: mode === 'split' ? requestRatio : 1 }}>
            <header className="border-b px-4 py-2 text-sm font-semibold">请求</header>
            <div className="flex-1 space-y-3 overflow-auto p-4">
              <div>
                <p className="text-xs font-medium text-muted-foreground">请求头</p>
                <pre className="mt-1 max-h-48 overflow-auto rounded bg-muted/60 p-3 text-xs">
{formatHeaders(detail?.request)}
                </pre>
              </div>
              <div>
                <p className="text-xs font-medium text-muted-foreground">请求体</p>
                <pre className="mt-1 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-muted/60 p-3 text-xs">
{formatBody(detail?.request?.body)}
                </pre>
              </div>
            </div>
          </section>
        ) : null}

        {mode === 'split' ? (
          <div
            role="separator"
            aria-orientation="vertical"
            className="flex w-1 cursor-col-resize items-stretch"
            onMouseDown={startResize}
          >
            <span className="mx-auto h-full w-px bg-border" />
          </div>
        ) : null}

        {showResponse ? (
          <section className="flex flex-col rounded-lg border bg-card" style={{ flex: mode === 'split' ? 1 - requestRatio : 1 }}>
            <header className="border-b px-4 py-2 text-sm font-semibold">响应</header>
            <div className="flex-1 space-y-3 overflow-auto p-4">
              <div>
                <p className="text-xs font-medium text-muted-foreground">响应头</p>
                <pre className="mt-1 max-h-48 overflow-auto rounded bg-muted/60 p-3 text-xs">
{formatHeaders(detail?.response)}
                </pre>
              </div>
              <div>
                <p className="text-xs font-medium text-muted-foreground">响应体</p>
                <pre className="mt-1 max-h-48 overflow-auto whitespace-pre-wrap rounded bg-muted/60 p-3 text-xs">
{formatBody(detail?.response?.body)}
                </pre>
              </div>
            </div>
          </section>
        ) : null}
      </div>
    );
  }, [detail?.request, detail?.response, entry, hasRequest, hasResponse, mode, requestRatio, startResize]);

  return (
    <div className="flex h-full flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-1">
          {DISPLAY_OPTIONS.map((option) => (
            <Button
              key={option.key}
              size="sm"
              variant={mode === option.key ? 'secondary' : 'outline'}
              className={cn('rounded-none first:rounded-l-md last:rounded-r-md')}
              onClick={() => setMode(option.key)}
            >
              {option.label}
            </Button>
          ))}
        </div>
        <div className="flex items-center gap-2">
          {copyState === 'success' ? <Badge variant="success">已复制</Badge> : null}
          {copyState === 'error' ? <Badge variant="destructive">复制失败</Badge> : null}
          <Button size="sm" variant="outline" onClick={handleCopy} disabled={loading}>
            复制为 curl
          </Button>
        </div>
      </div>

      <div className={cn('relative flex-1 rounded-lg border p-4', loading && 'opacity-70')}>{content}</div>
    </div>
  );
}
