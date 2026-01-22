import { useCallback, useEffect, useMemo, useState } from 'react';

import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';

import { Accordion, AccordionContent, AccordionItem, AccordionTrigger } from '@/components/ui/accordion';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { HttpBodyPanel, type BodyConfig, type BodyFormat } from '@/components/http-body-panel';
import { getTrafficTags } from '@/lib/traffic-tags';
import { cn } from '@/lib/utils';
import { HttpMessage, TrafficDetail, TrafficEntry } from '@/types/traffic';

const DISPLAY_OPTIONS = [
  { key: 'split', label: '并排显示' },
  { key: 'request', label: '仅请求' },
  { key: 'response', label: '仅响应' },
] as const;

const VIEW_TABS = [
  { key: 'http', label: 'HTTP' },
  { key: 'ai', label: 'AI' },
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

const formatJsonValue = (value?: unknown) => {
  if (value === undefined || value === null) return '';
  if (typeof value === 'string') return value;
  try {
    return JSON.stringify(value, null, 2);
  } catch (error) {
    return String(value);
  }
};

const MarkdownBlock = ({ value }: { value: string }) => (
  <div className="text-xs/relaxed text-foreground">
    <ReactMarkdown
      remarkPlugins={[remarkGfm]}
      components={{
        p: ({ children }) => <p className="mb-3 last:mb-0">{children}</p>,
        ul: ({ children }) => <ul className="mb-3 list-disc pl-5 last:mb-0">{children}</ul>,
        ol: ({ children }) => <ol className="mb-3 list-decimal pl-5 last:mb-0">{children}</ol>,
        li: ({ children }) => <li className="mb-1 last:mb-0">{children}</li>,
        a: ({ children, href }) => (
          <a className="text-primary underline underline-offset-4" href={href} target="_blank" rel="noreferrer">
            {children}
          </a>
        ),
        code: ({ className, children }) =>
          className ? (
            <code className="font-mono text-[11px]">{children}</code>
          ) : (
            <code className="rounded bg-muted px-1 py-0.5 font-mono text-[11px]">{children}</code>
          ),
        pre: ({ children }) => (
          <pre className="mb-3 overflow-auto rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-xs leading-relaxed text-foreground">
            {children}
          </pre>
        ),
        blockquote: ({ children }) => (
          <blockquote className="border-l-2 border-border/60 pl-3 text-muted-foreground">{children}</blockquote>
        ),
      }}
    >
      {value}
    </ReactMarkdown>
  </div>
);

export function RequestResponsePanel({ entry, detail, loading }: RequestResponsePanelProps) {
  const [mode, setMode] = useState<DisplayMode>('split');
  const [copyState, setCopyState] = useState<CopyState>('idle');
  const [requestTab, setRequestTab] = useState<ViewTab>('http');
  const [responseTab, setResponseTab] = useState<ViewTab>('http');
  const entryId = entry?.id;
  const hasAiTag = useMemo(() => getTrafficTags(entry).includes('ai'), [entry]);

  useEffect(() => {
    setCopyState(entryId ? 'idle' : 'idle');
    setRequestTab('http');
    setResponseTab('http');
  }, [entryId]);

  useEffect(() => {
    if (!hasAiTag) {
      if (requestTab === 'ai') {
        setRequestTab('http');
      }
      if (responseTab === 'ai') {
        setResponseTab('http');
      }
    }
  }, [hasAiTag, requestTab, responseTab]);

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
    const llmRequest = detail?.request?.llm?.request;
    const llmResponse = detail?.response?.llm?.response;
    const llmMeta = {
      provider: detail?.request?.llm?.provider ?? detail?.response?.llm?.provider,
      model: detail?.request?.llm?.model ?? detail?.response?.llm?.model,
      streaming: detail?.request?.llm?.streaming ?? detail?.response?.llm?.streaming,
    };
    const hasLLMMeta = Boolean(llmMeta.provider || llmMeta.model || llmMeta.streaming);
    const hasLLMRequest = Boolean(llmRequest?.prompt || llmRequest?.toolCalls || llmRequest?.tools);
    const hasLLMResponse = Boolean(llmResponse?.content || llmResponse?.toolCalls || llmResponse?.reasoning);

    const renderLLMBadges = () =>
      hasLLMMeta ? (
        <div className="flex flex-wrap items-center gap-1.5 text-[11px]">
          {llmMeta.provider ? <Badge variant="outline">Provider: {llmMeta.provider}</Badge> : null}
          {llmMeta.model ? <Badge variant="outline">Model: {llmMeta.model}</Badge> : null}
          {llmMeta.streaming ? <Badge variant="warning">SSE Streaming</Badge> : null}
        </div>
      ) : null;

    const toolLabelMap: Record<string, string> = {
      web_search: 'Web Search',
      websearch: 'Web Search',
      file_search: 'File Search',
      code_interpreter: 'Code Interpreter',
      browser: 'Browser',
      vision: 'Vision',
    };

    const getToolLabel = (tool: unknown) => {
      if (!tool) return 'Unknown Tool';
      if (typeof tool === 'string') return tool;
      if (typeof tool === 'object') {
        const typedTool = tool as { type?: string; name?: string; function?: { name?: string } };
        const rawType = typeof typedTool.type === 'string' ? typedTool.type : '';
        const normalized = rawType.toLowerCase();
        if (toolLabelMap[normalized]) return toolLabelMap[normalized];
        if (normalized === 'function' && typeof typedTool.name === 'string') return typedTool.name;
        if (normalized === 'function' && typeof typedTool.function?.name === 'string') return typedTool.function.name;
        if (typeof typedTool.name === 'string') return typedTool.name;
        if (rawType) return rawType;
      }
      return 'Unknown Tool';
    };

    const getToolsList = (tools?: unknown) => {
      if (!tools) return [] as unknown[];
      if (Array.isArray(tools)) return tools;
      if (typeof tools === 'object') return Object.values(tools as Record<string, unknown>);
      return [tools];
    };

    const getToolKey = (tool: unknown) => {
      if (typeof tool === 'string') return `tool-${tool}`;
      if (typeof tool === 'object' && tool) {
        const typedTool = tool as { type?: string; name?: string; function?: { name?: string } };
        const key = typedTool.name || typedTool.function?.name || typedTool.type;
        if (key) return `tool-${key}`;
      }
      return `tool-${getToolLabel(tool)}`;
    };

    const renderJsonPanel = (value?: unknown) => (
      <pre className="max-h-64 overflow-auto rounded-md border border-border/60 bg-muted/40 p-2 font-mono text-xs leading-relaxed text-foreground">
        {formatJsonValue(value)}
      </pre>
    );

    const renderLLMRequest = () => {
      if (!hasLLMRequest && !hasLLMMeta) {
        return (
          <div className="flex min-h-[160px] items-center justify-center rounded-md border border-dashed border-border/60 bg-muted/30 p-3 text-xs text-muted-foreground">
            暂无 AI 请求解析
          </div>
        );
      }

      return (
        <div className="flex flex-col gap-2">
          {renderLLMBadges()}
          {llmRequest?.prompt ? (
            <Card size="sm">
              <CardHeader className="border-b border-border/60">
                <CardTitle className="text-xs font-semibold text-muted-foreground">Prompt</CardTitle>
              </CardHeader>
              <CardContent>
                <MarkdownBlock value={llmRequest.prompt} />
              </CardContent>
            </Card>
          ) : null}
          {llmRequest?.tools ? (
            <div className="rounded-md border border-border/60 bg-muted/30 p-2">
              <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">Tools</div>
              <div className="mt-2 flex flex-wrap gap-1.5">
                {getToolsList(llmRequest.tools).length ? (
                  getToolsList(llmRequest.tools).map((tool) => (
                    <Badge key={getToolKey(tool)} variant="outline">
                      {getToolLabel(tool)}
                    </Badge>
                  ))
                ) : (
                  <span className="text-xs text-muted-foreground">No tools</span>
                )}
              </div>
            </div>
          ) : null}
          {llmRequest?.toolCalls ? (
            <Accordion type="multiple">
              <AccordionItem value="llm-request-tool-calls">
                <AccordionTrigger>Tool Calls</AccordionTrigger>
                <AccordionContent>{renderJsonPanel(llmRequest.toolCalls)}</AccordionContent>
              </AccordionItem>
            </Accordion>
          ) : null}
        </div>
      );
    };

    const renderLLMResponse = () => {
      if (!hasLLMResponse && !hasLLMMeta) {
        return (
          <div className="flex min-h-[160px] items-center justify-center rounded-md border border-dashed border-border/60 bg-muted/30 p-3 text-xs text-muted-foreground">
            暂无 AI 响应解析
          </div>
        );
      }

      return (
        <div className="flex flex-col gap-2">
          {renderLLMBadges()}
          {llmResponse?.content ? (
            <Card size="sm">
              <CardHeader className="border-b border-border/60">
                <CardTitle className="text-xs font-semibold text-muted-foreground">Content</CardTitle>
              </CardHeader>
              <CardContent>
                <MarkdownBlock value={llmResponse.content} />
              </CardContent>
            </Card>
          ) : null}
          {llmResponse?.reasoning || llmResponse?.toolCalls ? (
            <Accordion type="multiple">
              {llmResponse?.reasoning ? (
                <AccordionItem value="llm-response-reasoning">
                  <AccordionTrigger>思考过程</AccordionTrigger>
                  <AccordionContent>
                    <div className="rounded-md border border-border/60 bg-muted/40 p-2">
                      <MarkdownBlock value={llmResponse.reasoning} />
                    </div>
                  </AccordionContent>
                </AccordionItem>
              ) : null}
              {llmResponse?.toolCalls ? (
                <AccordionItem value="llm-response-tool-calls">
                  <AccordionTrigger>Tool Calls</AccordionTrigger>
                  <AccordionContent>{renderJsonPanel(llmResponse.toolCalls)}</AccordionContent>
                </AccordionItem>
              ) : null}
            </Accordion>
          ) : null}
        </div>
      );
    };

    const tabItems = hasAiTag ? VIEW_TABS : VIEW_TABS.filter((tab) => tab.key !== 'ai');
    const renderTabRow = (activeTab: ViewTab, onChange: (tab: ViewTab) => void) => (
      <ToggleGroup
        type="single"
        value={activeTab}
        onValueChange={(val) => {
          if (val) onChange(val as ViewTab);
        }}
      >
        {tabItems.map((tab) => (
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

    const renderRequest = () => (
      <div className="flex h-full min-h-0 flex-col gap-2">
        <div className="shrink-0">
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
        <HttpBodyPanel title="请求体" config={requestBodyConfig} className="flex-1 min-h-0" />
      </div>
    );

    const renderResponse = () => (
      <div className="flex h-full min-h-0 flex-col gap-2">
        <div className="shrink-0">
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
        <HttpBodyPanel title="响应体" config={responseBodyConfig} className="flex-1 min-h-0" />
      </div>
    );

    const requestSection = (
      <section className="flex h-full min-h-0 flex-col border border-border/60 bg-card/80">
        <header className="flex items-center justify-between gap-2 border-b border-border/60 bg-background/40 px-2 py-1 text-[11px]">
          <span className="font-semibold text-muted-foreground">请求</span>
          {renderTabRow(requestTab, setRequestTab)}
        </header>
        <div className="flex-1 min-h-0 overflow-auto p-2">
          {requestTab === 'http'
            ? renderRequest()
            : requestTab === 'ai'
              ? renderLLMRequest()
              : renderPlaceholder('请求')}
        </div>
      </section>
    );

    const responseSection = (
      <section className="flex h-full min-h-0 flex-col border border-border/60 bg-card/80">
        <header className="flex items-center justify-between gap-2 border-b border-border/60 bg-background/40 px-2 py-1 text-[11px]">
          <span className="font-semibold text-muted-foreground">响应</span>
          {renderTabRow(responseTab, setResponseTab)}
        </header>
        <div className="flex-1 min-h-0 overflow-auto p-2">
          {responseTab === 'http'
            ? renderResponse()
            : responseTab === 'ai'
              ? renderLLMResponse()
              : renderPlaceholder('响应')}
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
          <ResizableHandle className="bg-border/60 transition-colors hover:bg-accent/60 focus:outline-none" />
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
  }, [detail?.request, detail?.response, entry, hasAiTag, hasRequest, hasResponse, mode, requestTab, responseTab]);

  return (
    <div className="flex h-full w-full min-w-0 flex-col overflow-x-hidden">
      <div className="flex shrink-0 min-w-0 flex-nowrap items-center justify-between gap-3  px-3 py-1.5 text-xs">
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
