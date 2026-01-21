import { useCallback, useEffect, useMemo } from 'react';

import { RequestResponsePanel } from '@/components/request-response-panel';
import { TrafficDataTable } from '@/components/traffic-data-table';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ResizableHandle, ResizablePanel, ResizablePanelGroup } from '@/components/ui/resizable';
import { useTrafficStream } from '@/hooks/use-traffic-stream';
import { useTrafficStore } from '@/stores/use-traffic-store';
import { TrafficEntry } from '@/types/traffic';

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

      <section className="flex h-full min-h-0 w-full min-w-0 flex-1 flex-col gap-3 overflow-x-hidden">
        {/* 主内容区域 - 表格 */}
        <div className="flex min-h-0 w-full min-w-0 flex-1 overflow-x-hidden">
          <ResizablePanelGroup
            orientation="vertical"
            className="h-full min-h-0 w-full min-w-0 overflow-hidden overflow-x-hidden"
          >
            <ResizablePanel
              defaultSize={62}
              minSize={35}
              className="flex h-full min-w-0 flex-col rounded-none border border-border/60 bg-card/70"
            >
              <div className="flex flex-wrap items-center justify-between gap-2 border-b border-border/60 px-3 py-1.5 text-xs">
                <div className="flex flex-wrap items-center gap-2 text-muted-foreground">
                  <div className="flex items-center gap-2">
                    <span className="uppercase">Traffic Requests</span>
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
                  <Button
                    size="sm"
                    variant="destructive"
                    onClick={handleClear}
                    disabled={loading}
                    className="h-6 text-xs"
                  >
                    清空
                  </Button>
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={handleReconnect}
                    disabled={loading}
                    className="h-6 text-xs"
                  >
                    重新连接
                  </Button>
                </div>
              </div>
              <div className="flex-1 min-w-0 overflow-y-auto overflow-x-hidden">
                <TrafficDataTable
                  data={list}
                  selectedId={selectedId}
                  onSelect={selectEntry}
                  emptyMessage={loading ? '加载中…' : '暂无流量记录'}
                />
              </div>
            </ResizablePanel>
            <ResizableHandle className="bg-border/60" />
            <ResizablePanel
              defaultSize={38}
              minSize={25}
              className="flex h-full min-w-0 flex-col rounded-none border border-border/60 bg-card/70"
            >
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
