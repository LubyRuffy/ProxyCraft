import { useCallback, useEffect, useMemo } from 'react';

import { RequestResponsePanel } from '@/components/request-response-panel';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
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
  if (status >= 500) return 'text-red-500';
  if (status >= 400) return 'text-amber-500';
  if (status >= 200) return 'text-emerald-500';
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
    <section className="flex h-full flex-1 flex-col gap-4 px-6 py-4">
      {/* 工具栏区域 */}
      <div className="flex items-center justify-between gap-4 border-b pb-3">
        <div className="flex items-center gap-4">
          <h1 className="text-lg font-semibold">流量列表</h1>
          <div className="flex gap-2">
            <Button size="sm" onClick={handleRefresh} disabled={loading}>
              {loading ? '处理中…' : '刷新'}
            </Button>
            <Button size="sm" variant="destructive" onClick={handleClear} disabled={loading}>
              清空
            </Button>
            <Button size="sm" variant="secondary" onClick={handleReconnect} disabled={loading}>
              重新连接
            </Button>
          </div>
        </div>
        <div className="flex items-center gap-3 text-sm text-muted-foreground">
          <Badge variant={connected ? 'success' : 'warning'}>
            {connected ? 'WebSocket 已连接' : 'WebSocket 未连接'}
          </Badge>
          <span>最后更新：{lastUpdated}</span>
        </div>
      </div>

      {/* 错误提示 */}
      {error ? (
        <div className="rounded-lg border border-destructive/50 bg-destructive/5 p-4 text-sm text-destructive-foreground">
          {error}
        </div>
      ) : null}

      {/* 主内容区域 - 表格 */}
      <div className="flex-1 overflow-hidden">
        <div className="h-full overflow-auto rounded-lg border">
          <table className="min-w-full text-sm">
            <thead className="bg-muted/50 text-xs uppercase text-muted-foreground sticky top-0">
              <tr>
                <th className="px-4 py-2 text-left font-medium w-16">方法</th>
                <th className="px-4 py-2 text-left font-medium min-w-32">Host</th>
                <th className="px-4 py-2 text-left font-medium">Path</th>
                <th className="px-4 py-2 text-left font-medium w-16">Code</th>
                <th className="px-4 py-2 text-left font-medium w-24">MIME</th>
                <th className="px-4 py-2 text-left font-medium w-20">Size</th>
                <th className="px-4 py-2 text-left font-medium w-20">Cost</th>
                <th className="px-4 py-2 text-left font-medium w-24">Tags</th>
              </tr>
            </thead>
            <tbody>
              {list.map((entry) => (
                <tr
                  key={entry.id}
                  onClick={() => selectEntry(entry.id)}
                  className={cn(
                    'cursor-pointer transition-colors hover:bg-muted/70',
                    selectedId === entry.id && 'bg-secondary/40'
                  )}
                >
                  <td className="px-4 py-2 font-medium">{entry.method}</td>
                  <td className="px-4 py-2 truncate text-muted-foreground">{entry.host}</td>
                  <td className="px-4 py-2 truncate">{entry.path}</td>
                  <td className={cn('px-4 py-2 font-mono', statusTint(entry.statusCode))}>
                    {entry.statusCode}
                  </td>
                  <td className="px-4 py-2 truncate text-muted-foreground">{entry.contentType}</td>
                  <td className="px-4 py-2 text-muted-foreground">{formatBytes(entry.contentSize)}</td>
                  <td className="px-4 py-2 text-muted-foreground">{entry.duration} ms</td>
                  <td className="px-4 py-2">
                    <div className="flex gap-1">
                      {entry.isHTTPS ? <Badge variant="success">HTTPS</Badge> : null}
                      {entry.isSSE ? <Badge variant="warning">SSE</Badge> : null}
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {list.length === 0 ? (
            <div className="flex items-center justify-center p-20 text-sm text-muted-foreground">
              {loading ? '加载中…' : '暂无流量记录'}
            </div>
          ) : null}
        </div>
      </div>

      {/* 详情区域 */}
      <div className="h-80 border rounded-lg overflow-hidden">
        <div className="flex h-full flex-col">
          <div className="flex items-center justify-between border-b px-4 py-2">
            <h2 className="text-base font-semibold">请求 / 响应详情</h2>
            {selectedId && (
              <div className="flex items-center gap-2 text-sm">
                <Badge variant="outline">ID: {selectedId}</Badge>
                {loading ? <Badge variant="warning">加载中…</Badge> : null}
              </div>
            )}
          </div>
          <div className="flex-1 p-4">
            {selectedId ? (
              <RequestResponsePanel entry={selectedEntry ?? undefined} detail={detail} loading={loading} />
            ) : (
              <div className="flex h-full items-center justify-center rounded-lg border border-dashed text-sm text-muted-foreground">
                请选择列表中的一条流量记录以查看详情。
              </div>
            )}
          </div>
        </div>
      </div>
    </section>
  );
}
