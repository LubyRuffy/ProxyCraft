import { useCallback, useEffect } from 'react';

import {
  clearTrafficRemote,
  fetchTrafficDetail,
  fetchTrafficEntries,
} from '@/services/traffic-service';
import { trafficSocket } from '@/services/websocket';
import { useTrafficStore } from '@/stores/use-traffic-store';

export function useTrafficStream() {
  const setEntries = useTrafficStore((state) => state.setEntries);
  const addOrUpdateEntry = useTrafficStore((state) => state.addOrUpdateEntry);
  const clearEntries = useTrafficStore((state) => state.clearEntries);
  const setConnected = useTrafficStore((state) => state.setConnected);
  const setError = useTrafficStore((state) => state.setError);
  const setTransport = useTrafficStore((state) => state.setTransport);
  const mergeDetail = useTrafficStore((state) => state.mergeDetail);
  const clearDetail = useTrafficStore((state) => state.clearDetail);
  const selectedId = useTrafficStore((state) => state.selectedId);
  const entries = useTrafficStore((state) => state.entries);
  const selectEntry = useTrafficStore((state) => state.selectEntry);
  const setLoading = useTrafficStore((state) => state.setLoading);

  useEffect(() => {
    trafficSocket.connect();

    const cleanups = [
      trafficSocket.onConnect(() => {
        setConnected(true);
        setError(null);
        setTransport(trafficSocket.getTransport());
        trafficSocket.requestTrafficEntries(true);
      }),
      trafficSocket.onDisconnect((reason) => {
        setConnected(false);
        setTransport('disconnected');
        setError(`WebSocket已断开: ${reason}`);
      }),
      trafficSocket.onConnectError((error) => {
        setConnected(false);
        setTransport('disconnected');
        setError(`WebSocket连接错误: ${error.message}`);
      }),
      trafficSocket.onTrafficEntries((incoming) => {
        setEntries(incoming);
        setError(null);
        setLoading(false);
      }),
      trafficSocket.onNewTrafficEntry((entry) => {
        addOrUpdateEntry(entry);
      }),
      trafficSocket.onTrafficClear(() => {
        clearEntries();
        clearDetail();
        selectEntry(null);
        setError(null);
        setLoading(false);
      }),
      trafficSocket.onRequestDetails((request) => {
        mergeDetail({ request });
        setError(null);
        setLoading(false);
      }),
      trafficSocket.onResponseDetails((response) => {
        mergeDetail({ response });
        setError(null);
        setLoading(false);
      }),
    ];

    return () => {
      cleanups.forEach((cleanup) => cleanup());
      trafficSocket.disconnect();
    };
  }, [
    addOrUpdateEntry,
    clearDetail,
    clearEntries,
    mergeDetail,
    selectEntry,
    setConnected,
    setEntries,
    setError,
    setLoading,
    setTransport,
  ]);

  useEffect(() => {
    const timer = window.setInterval(() => {
      setTransport(trafficSocket.getTransport());
      const connected = trafficSocket.isConnected();
      setConnected(connected);
      if (connected) {
        setError(null);
      }
    }, 2000);

    return () => {
      window.clearInterval(timer);
    };
  }, [setConnected, setError, setTransport]);

  useEffect(() => {
    if (!selectedId) {
      clearDetail();
      return;
    }

    if (trafficSocket.isConnected()) {
      setLoading(true);
      trafficSocket.requestRequestDetails(selectedId);
      trafficSocket.requestResponseDetails(selectedId);
      return;
    }

    setLoading(true);
    fetchTrafficDetail(selectedId)
      .then((detail) => {
        if (!detail) {
          clearDetail();
          return;
        }
        mergeDetail(detail);
      })
      .catch((error) => {
        setError(`加载详情失败: ${error instanceof Error ? error.message : '未知错误'}`);
        clearDetail();
      })
      .finally(() => setLoading(false));
  }, [clearDetail, mergeDetail, selectedId, setError, setLoading]);

  useEffect(() => {
    const entry = entries.find((item) => item.id === selectedId);
    if (!selectedId || !entry?.isSSE || entry.isSSECompleted) {
      return undefined;
    }

    const timer = window.setInterval(() => {
      if (trafficSocket.isConnected()) {
        trafficSocket.requestRequestDetails(selectedId);
        trafficSocket.requestResponseDetails(selectedId);
      } else {
        fetchTrafficDetail(selectedId)
          .then((detail) => {
            if (!detail) {
              return;
            }
            mergeDetail(detail);
          })
          .catch((error) => {
            setError(`加载详情失败: ${error instanceof Error ? error.message : '未知错误'}`);
          });
      }
    }, 1000);

    return () => window.clearInterval(timer);
  }, [entries, mergeDetail, selectedId, setError]);

  const refresh = useCallback(() => {
    setLoading(true);
    if (trafficSocket.isConnected()) {
      trafficSocket.requestTrafficEntries(true);
      return;
    }
    fetchTrafficEntries()
      .then((list) => {
        setEntries(list);
        setError(null);
      })
      .catch((error) => {
        setError(`获取流量数据失败: ${error instanceof Error ? error.message : '未知错误'}`);
      })
      .finally(() => {
        setLoading(false);
      });
  }, [setEntries, setError, setLoading]);

  const reconnect = useCallback(() => {
    setLoading(true);
    trafficSocket.reconnect();
  }, [setLoading]);

  const clearRemoteTraffic = useCallback(async () => {
    setLoading(true);
    if (trafficSocket.isConnected()) {
      trafficSocket.requestClearTraffic();
      selectEntry(null);
      return true;
    }
    const cleared = await clearTrafficRemote();
    if (cleared) {
      clearEntries();
      clearDetail();
      selectEntry(null);
      setError(null);
    }
    setLoading(false);
    return cleared;
  }, [clearDetail, clearEntries, selectEntry, setError, setLoading]);

  return {
    refresh,
    reconnect,
    clearRemoteTraffic,
  };
}
