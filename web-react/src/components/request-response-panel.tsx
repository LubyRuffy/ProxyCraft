import { useCallback, useEffect, useMemo, useState } from 'react';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { HttpBodyPanel, type BodyConfig, type BodyFormat } from '@/components/http-body-panel';
import { cn } from '@/lib/utils';
import { HttpMessage, TrafficDetail, TrafficEntry } from '@/types/traffic';

const DISPLAY_OPTIONS = [
  { key: 'split', label: '并排显示' },
  { key: 'request', label: '仅请求' },
  { key: 'response', label: '仅响应' },
] as const;

const VIEW_TABS = [
  { key: 'pretty', label: 'Pretty' },
  { key: 'raw', label: 'Raw' },
  { key: 'hex', label: 'Hex' },
  { key: 'render', label: 'Render' },
] as const;

type DisplayMode = (typeof DISPLAY_OPTIONS)[number]['key'];
type ViewTab = (typeof VIEW_TABS)[number]['key'];

type CopyState = 'idle' | 'success' | 'error';

type RequestResponsePanelProps = {
  entry?: TrafficEntry | null;
  detail?: TrafficDetail;
  loading?: boolean;
};

const formatHeaderEntries = (message?: HttpMessage) => {
  if (!message?.headers) return [] as Array<[string, string]>;
  return Object.entries(message.headers).filter(([key, value]) => key && value !== undefined && value !== null);
};

const buildRequestLine = (entry?: TrafficEntry | null) => {
  if (!entry) return 'REQUEST';
  const path = entry.path || '/';
  return `${entry.method} ${path} HTTP/1.1`;
};

const buildResponseLine = (entry?: TrafficEntry | null) => {
  if (!entry) return 'RESPONSE';
  return `HTTP/1.1 ${entry.statusCode || 0}`;
};

const getBodyString = (body?: unknown) => {
  if (body === undefined || body === null || body === '') return '';
  if (typeof body === 'string') return body;
  try {
    return JSON.stringify(body, null, 2);
  } catch (error) {
    return String(body);
  }
};

const getContentType = (message?: HttpMessage) => {
  const headers = message?.headers;
  if (!headers) return '';
  const header = Object.entries(headers).find(([key]) => key.toLowerCase() === 'content-type');
  if (!header || header[1] === undefined || header[1] === null) return '';
  return String(header[1]).toLowerCase();
};

const detectBodyFormat = (body?: unknown, message?: HttpMessage): BodyFormat => {
  if (body && typeof body !== 'string') return 'json';
  const contentType = getContentType(message);
  if (contentType.includes('json') || contentType.includes('+json')) return 'json';
  if (contentType.includes('html') || contentType.includes('xhtml')) return 'html';
  if (contentType.includes('xml') || contentType.includes('+xml')) return 'xml';
  if (contentType.includes('yaml') || contentType.includes('x-yaml')) return 'yaml';
  if (contentType.includes('javascript') || contentType.includes('ecmascript')) return 'javascript';
  if (contentType.includes('text/event-stream')) return 'sse';
  if (contentType.includes('text/plain')) return 'text';

  const text = typeof body === 'string' ? body.trim() : '';
  if (text.startsWith('<?xml')) return 'xml';
  if (text.startsWith('<')) return 'html';
  if (text.startsWith('---')) return 'yaml';
  if (text.startsWith('{') || text.startsWith('[')) return 'json';
  return 'text';
};
const buildEditorConfig = (body?: unknown, message?: HttpMessage): BodyConfig => {
  const rawBody = getBodyString(body);
  if (!rawBody) return { value: '', format: 'text' as BodyFormat };

  const format = detectBodyFormat(body, message);
  let value = rawBody;

  if (format === 'json') {
    if (typeof body !== 'string') {
      value = getBodyString(body);
    } else {
      try {
        const parsed = JSON.parse(rawBody);
        value = JSON.stringify(parsed, null, 2);
      } catch (error) {
        value = rawBody;
      }
    }
  }

  return { value, format };
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
  const [requestTab, setRequestTab] = useState<ViewTab>('pretty');
  const [responseTab, setResponseTab] = useState<ViewTab>('pretty');
  const entryId = entry?.id;

  useEffect(() => {
    setCopyState(entryId ? 'idle' : 'idle');
    setRequestTab('pretty');
    setResponseTab('pretty');
  }, [entryId]);

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

  const hasRequest = Boolean(detail?.request);
  const hasResponse = Boolean(detail?.response);

  const content = useMemo(() => {
      if (!entry || (!hasRequest && !hasResponse)) {
        return (
          <div className="flex h-full items-center justify-center rounded-none border border-dashed border-border/60 p-6 text-sm text-muted-foreground">
            请选择列表中的一条流量记录以查看详情。
          </div>
        );
      }

    const showRequest = mode === 'split' || mode === 'request';
    const showResponse = mode === 'split' || mode === 'response';

    const requestHeaders = formatHeaderEntries(detail?.request);
    const responseHeaders = formatHeaderEntries(detail?.response);

    const renderTabRow = (activeTab: ViewTab, onChange: (tab: ViewTab) => void) => (
      <ToggleGroup
        type="single"
        value={activeTab}
        onValueChange={(val) => {
          if (val) onChange(val as ViewTab);
        }}
      >
        {VIEW_TABS.map((tab) => (
          <ToggleGroupItem
            key={tab.key}
            value={tab.key}
          >
            {tab.label}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    );

    const renderPlaceholder = (label: string) => (
      <div className="flex h-full min-h-[160px] items-center justify-center rounded-md border border-dashed border-border/60 bg-muted/30 p-3 font-mono text-xs text-muted-foreground">
        {label} 视图暂未实现
      </div>
    );

    const requestBodyConfig = buildEditorConfig(detail?.request?.body, detail?.request);
    const responseBodyConfig = buildEditorConfig(detail?.response?.body, detail?.response);

    const renderRequestPretty = () => (
      <div className="space-y-2">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-muted-foreground">请求头</p>
          <div className="mt-1 max-h-44 overflow-auto rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-[11px] leading-relaxed">
            <div className="text-foreground">{buildRequestLine(entry)}</div>
            {requestHeaders.length ? (
              requestHeaders.map(([key, value]) => (
                <div key={`${key}-${value}`} className="flex min-w-0 flex-wrap">
                  <span className="text-primary">{key}:</span>
                  <span className="ml-1 min-w-0 break-words text-foreground">{value}</span>
                </div>
              ))
            ) : (
              <div className="text-muted-foreground">No headers</div>
            )}
          </div>
        </div>
        <HttpBodyPanel title="请求体" config={requestBodyConfig} />
      </div>
    );

    const renderResponsePretty = () => (
      <div className="space-y-2">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.2em] text-muted-foreground">响应头</p>
          <div className="mt-1 max-h-44 overflow-auto rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-[11px] leading-relaxed">
            <div className="text-foreground">{buildResponseLine(entry)}</div>
            {responseHeaders.length ? (
              responseHeaders.map(([key, value]) => (
                <div key={`${key}-${value}`} className="flex min-w-0 flex-wrap">
                  <span className="text-primary">{key}:</span>
                  <span className="ml-1 min-w-0 break-words text-foreground">{value}</span>
                </div>
              ))
            ) : (
              <div className="text-muted-foreground">No headers</div>
            )}
          </div>
        </div>
        <HttpBodyPanel title="响应体" config={responseBodyConfig} />
      </div>
    );

    const requestSection = (
      <section className="flex h-full min-h-0 flex-col rounded-lg border border-border/60 bg-card/80">
        <header className="flex items-center justify-between gap-2 bg-background/40 px-2 py-1 text-[11px]">
          <span className="font-semibold text-muted-foreground">请求</span>
          {renderTabRow(requestTab, setRequestTab)}
        </header>
        <div className="flex-1 min-h-0 overflow-auto p-2">
          {requestTab === 'pretty' ? renderRequestPretty() : renderPlaceholder('请求')}
        </div>
      </section>
    );

    const responseSection = (
      <section className="flex h-full min-h-0 flex-col rounded-lg border border-border/60 bg-card/80">
        <header className="flex items-center justify-between gap-2 border-b border-border/60 bg-background/40 px-2 py-1 text-[11px]">
          <span className="font-semibold text-muted-foreground">响应</span>
          {renderTabRow(responseTab, setResponseTab)}
        </header>
        <div className="flex-1 min-h-0 overflow-auto p-2">
          {responseTab === 'pretty' ? renderResponsePretty() : renderPlaceholder('响应')}
        </div>
      </section>
    );

    if (mode === 'split') {
      return (
        <ResizablePanelGroup
          orientation="horizontal"
          className="min-h-[320px] w-full min-w-0 overflow-hidden overflow-x-hidden"
        >
          <ResizablePanel defaultSize={50} minSize={20} className="min-h-0 min-w-0">
            {requestSection}
          </ResizablePanel>
          <ResizableHandle className="bg-border/70" />
          <ResizablePanel defaultSize={50} minSize={20} className="min-h-0 min-w-0">
            {responseSection}
          </ResizablePanel>
        </ResizablePanelGroup>
      );
    }

    return (
      <div className="flex min-h-[320px] w-full min-w-0 flex-col gap-2 overflow-x-hidden">
        {showRequest ? requestSection : null}
        {showResponse ? responseSection : null}
      </div>
    );
  }, [detail?.request, detail?.response, entry, hasRequest, hasResponse, mode, requestTab, responseTab]);

  return (
    <div className="flex h-full w-full min-w-0 flex-col overflow-x-hidden">
      <div className="flex shrink-0 min-w-0 flex-nowrap items-center justify-between gap-3 border-b border-border/60 px-3 py-1.5 text-xs">
        <div className="flex min-w-0 flex-1 flex-nowrap items-center gap-3">
          <div className="min-w-0">
            <p className="uppercase racking-[0.25em] text-muted-foreground">Inspector</p>
          </div>
          {entryId ? (
            <div className="flex shrink-0 items-center gap-1.5 text-xs">
              <Badge variant="outline">ID: {entryId}</Badge>
              {loading ? <Badge variant="warning">加载中…</Badge> : null}
            </div>
          ) : null}
        </div>
        <div className="flex shrink-0 flex-nowrap items-center gap-2">
          <ToggleGroup
            type="single"
            variant="outline"
            size="sm"
            value={mode}
            onValueChange={(next) => {
              if (next) {
                setMode(next as DisplayMode);
              }
            }}
          >
            {DISPLAY_OPTIONS.map((option) => (
              <ToggleGroupItem
                key={option.key}
                value={option.key}
                aria-label={option.label}
              >
                {option.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
          <div className="flex shrink-0 items-center gap-1.5">
            {copyState === 'success' ? <Badge variant="success">已复制</Badge> : null}
            {copyState === 'error' ? <Badge variant="destructive">复制失败</Badge> : null}
            <Button size="sm" variant="outline" onClick={handleCopy} disabled={loading} className="h-6 text-xs">
              复制为 curl
            </Button>
          </div>
        </div>
      </div>

      <div className="flex-1 min-h-0 min-w-0">
        <div
          className={cn(
            'relative flex h-full min-h-0 min-w-0 overflow-hidden rounded-none',
            loading && 'opacity-70'
          )}
        >
          {content}
        </div>
      </div>
    </div>
  );
}
