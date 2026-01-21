import { useCallback, useEffect, useMemo } from 'react';

import { RequestResponsePanel } from '@/components/request-response-panel';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable';
import { useTrafficStream } from '@/hooks/use-traffic-stream';
import { cn } from '@/lib/utils';
import { useTrafficStore } from '@/stores/use-traffic-store';
import { TrafficEntry } from '@/types/traffic';

const formatBytes = (bytes: number) => {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const power = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${(bytes / 1024 ** power).toFixed(power === 0 ? 0 : 1)} ${units[power]}`;
};

const statusTint = (status: number) => {
  if (status >= 500) return 'text-destructive';
  if (status >= 400) return 'text-accent';
  if (status >= 200) return 'text-primary';
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

  const { refresh, reconnect, clearRemoteTraffic } = useTrafficStream();

  useEffect(() => {
    if (entries.length === 0) {
      refresh();
    }
  }, [entries.length, refresh]);

  const list: TrafficEntry[] = useMemo(() => entries ?? [], [entries]);
  const selectedEntry = useMemo(
    () => list.find((item) => item.id === selectedId) ?? null,
    [list, selectedId]
  );
  const lastUpdated = useMemo(() => {
    if (!list.length) return '未同步';
    const timestamps = list
      .map((entry) => entry.endTime || entry.startTime)
      .filter(Boolean)
      .map((value) => new Date(value as string).getTime());
    if (!timestamps.length) return '未知';
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

  return (
    <div className="flex h-full min-h-0 w-full min-w-0 flex-1 flex-col overflow-x-hidden">
      {/* 错误提示 */}
      {error ? (
        <div className="px-4 pt-3">
          <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-4 text-sm text-destructive-foreground">
            {error}
          </div>
        </div>
      ) : null}

      <section className="flex h-full min-h-0 w-full min-w-0 flex-1 flex-col gap-3 overflow-x-hidden px-4 py-3">
        {/* 主内容区域 - 表格 */}
        <div className="flex min-h-0 w-full min-w-0 flex-1 overflow-x-hidden">
          <ResizablePanelGroup
            orientation="vertical"
            className="h-full min-h-0 w-full min-w-0 overflow-hidden overflow-x-hidden"
          >
            <ResizablePanel
              defaultSize={62}
              minSize={35}
              className="flex h-full min-w-0 flex-col rounded-xl border border-border/60 bg-card/70"
            >
            <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 px-3 py-1.5 text-xs">
              <div className="flex flex-wrap items-center gap-2 text-muted-foreground">
                <div className="flex items-center gap-2">
                  <span className="text-[11px] uppercase tracking-[0.3em]">Traffic</span>
                  <span className="text-xs font-semibold text-foreground">流量列表</span>
                </div>
                <div className="flex items-center gap-2">
                  <span className="text-[11px] uppercase tracking-[0.25em]">Requests</span>
                  <Badge variant="secondary">{list.length}</Badge>
                </div>
                <div className="flex flex-wrap items-center gap-1.5">
                  <Badge variant={connected ? 'success' : 'warning'}>
                    {connected ? 'WebSocket 已连接' : 'WebSocket 未连接'}
                  </Badge>
                  <span>最后更新：{lastUpdated}</span>
                </div>
              </div>
              <div className="flex flex-wrap items-center gap-1.5">
                <Button size="sm" onClick={handleRefresh} disabled={loading} className="h-6 text-xs">
                  {loading ? '处理中…' : '刷新'}
                </Button>
                <Button size="sm" variant="destructive" onClick={handleClear} disabled={loading} className="h-6 text-xs">
                  清空
                </Button>
                <Button size="sm" variant="secondary" onClick={handleReconnect} disabled={loading} className="h-6 text-xs">
                  重新连接
                </Button>
              </div>
            </div>
            <div className="flex-1 min-w-0 overflow-y-auto overflow-x-hidden">
              <table className="w-full min-w-0 table-fixed text-xs">
                <thead className="sticky top-0 border-b border-border/60 bg-muted/40 text-[11px] uppercase tracking-[0.2em] text-muted-foreground">
                  <tr>
                    <th className="w-16 px-2.5 py-1.5 text-left font-medium">方法</th>
                    <th className="w-28 px-2.5 py-1.5 text-left font-medium">Process</th>
                    <th className="w-36 px-2.5 py-1.5 text-left font-medium">Host</th>
                    <th className="px-2.5 py-1.5 text-left font-medium">Path</th>
                    <th className="w-16 px-2.5 py-1.5 text-left font-medium">Code</th>
                    <th className="w-24 px-2.5 py-1.5 text-left font-medium">MIME</th>
                    <th className="w-20 px-2.5 py-1.5 text-left font-medium">Size</th>
                    <th className="w-20 px-2.5 py-1.5 text-left font-medium">Cost</th>
                    <th className="w-24 px-2.5 py-1.5 text-left font-medium">Tags</th>
                  </tr>
                </thead>
                <tbody>
                  {list.map((entry) => (
                    <tr
                      key={entry.id}
                      onClick={() => selectEntry(entry.id)}
                      className={cn(
                        'cursor-pointer border-b border-border/60 transition-colors hover:bg-muted/50',
                        selectedId === entry.id && 'bg-secondary/30'
                      )}
                    >
                      <td className="px-2.5 py-1.5 font-medium">{entry.method}</td>
                      <td className="min-w-0 px-2.5 py-1.5">
                        <div className="flex min-w-0 items-center gap-2 text-muted-foreground">
                          {entry.processIcon ? (
                            <img
                              src={entry.processIcon}
                              alt={`${entry.processName || 'Process'} icon`}
                              className="h-4 w-4 rounded-[4px] object-cover"
                            />
                          ) : (
                            <div
                              role="img"
                              aria-label="Process icon placeholder"
                              className="flex h-4 w-4 items-center justify-center rounded-[4px] border border-border/60 bg-muted/60 text-[10px] font-semibold text-muted-foreground"
                            >
                              {(entry.processName || '?').slice(0, 1).toUpperCase()}
                            </div>
                          )}
                          <span className="min-w-0 truncate">{entry.processName || '-'}</span>
                        </div>
                      </td>
                      <td className="min-w-0 px-2.5 py-1.5 truncate text-muted-foreground">{entry.host}</td>
                      <td className="min-w-0 px-2.5 py-1.5 truncate">{entry.path}</td>
                      <td className={cn('px-2.5 py-1.5 font-mono', statusTint(entry.statusCode))}>
                        {entry.statusCode}
                      </td>
                      <td className="px-2.5 py-1.5 truncate text-muted-foreground">{entry.contentType}</td>
                      <td className="px-2.5 py-1.5 text-muted-foreground">{formatBytes(entry.contentSize)}</td>
                      <td className="px-2.5 py-1.5 text-muted-foreground">{entry.duration} ms</td>
                      <td className="min-w-0 px-2.5 py-1.5">
                        <div className="flex min-w-0 flex-wrap gap-1">
                          {entry.isHTTPS ? <Badge variant="success">HTTPS</Badge> : null}
                          {entry.isSSE ? <Badge variant="warning">SSE</Badge> : null}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
              {list.length === 0 ? (
                <div className="flex items-center justify-center p-8 text-sm text-muted-foreground">
                  {loading ? '加载中…' : '暂无流量记录'}
                </div>
              ) : null}
            </div>
          </ResizablePanel>
          <ResizableHandle
            withHandle
            className="bg-border/60"
          />
            <ResizablePanel defaultSize={38} minSize={25} className="flex h-full min-w-0 flex-col rounded-xl border border-border/60 bg-card/70">
              <div className="flex-1 min-w-0 overflow-x-hidden">
                <RequestResponsePanel entry={selectedEntry ?? undefined} detail={detail} loading={loading} />
              </div>
            </ResizablePanel>
          </ResizablePanelGroup>
        </div>
      </section>
    </div>
  );
}
