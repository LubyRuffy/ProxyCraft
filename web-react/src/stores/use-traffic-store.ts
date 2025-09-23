import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

import { TrafficDetail, TrafficEntry } from '@/types/traffic';

export type SettingsState = {
  // WebSocket连接设置
  autoReconnect: boolean;
  reconnectInterval: number;
  maxReconnectAttempts: number;

  // 显示设置
  entriesPerPage: number;
  showOnlyHttps: boolean;
  showOnlySse: boolean;

  // 数据保存设置
  autoSaveHar: boolean;
  harSaveInterval: number;

  // 过滤设置
  filterHost: string;
  filterMethod: 'all' | 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH';

  // 主题设置
  theme: 'light' | 'dark' | 'auto';

  setAutoReconnect: (autoReconnect: boolean) => void;
  setReconnectInterval: (interval: number) => void;
  setMaxReconnectAttempts: (max: number) => void;
  setEntriesPerPage: (count: number) => void;
  setShowOnlyHttps: (show: boolean) => void;
  setShowOnlySse: (show: boolean) => void;
  setAutoSaveHar: (autoSave: boolean) => void;
  setHarSaveInterval: (interval: number) => void;
  setFilterHost: (host: string) => void;
  setFilterMethod: (method: 'all' | 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH') => void;
  setTheme: (theme: 'light' | 'dark' | 'auto') => void;
  resetSettings: () => void;
};

export type TrafficState = {
  entries: TrafficEntry[];
  selectedId: string | null;
  detail?: TrafficDetail;
  loading: boolean;
  error: string | null;
  connected: boolean;
  transport: string;
  setEntries: (entries: TrafficEntry[]) => void;
  addOrUpdateEntry: (entry: TrafficEntry) => void;
  selectEntry: (id: string | null) => void;
  setDetail: (detail?: TrafficDetail) => void;
  mergeDetail: (partial: Partial<TrafficDetail>) => void;
  clearDetail: () => void;
  setLoading: (loading: boolean) => void;
  setError: (error: string | null) => void;
  setConnected: (connected: boolean) => void;
  setTransport: (transport: string) => void;
  clearEntries: () => void;
};

// 默认设置值
const defaultSettings: Omit<SettingsState, keyof { setAutoReconnect: any; setReconnectInterval: any; setMaxReconnectAttempts: any; setEntriesPerPage: any; setShowOnlyHttps: any; setShowOnlySse: any; setAutoSaveHar: any; setHarSaveInterval: any; setFilterHost: any; setFilterMethod: any; setTheme: any; resetSettings: any }> = {
  autoReconnect: true,
  reconnectInterval: 5,
  maxReconnectAttempts: 10,
  entriesPerPage: 50,
  showOnlyHttps: false,
  showOnlySse: false,
  autoSaveHar: false,
  harSaveInterval: 30,
  filterHost: '',
  filterMethod: 'all',
  theme: 'auto' as const,
};

// 创建设置store
const settingsStore = create<SettingsState>()(
  devtools(
    (set) => ({
      ...defaultSettings,
      setAutoReconnect: (autoReconnect) => set({ autoReconnect }),
      setReconnectInterval: (interval) => set({ reconnectInterval: Math.max(1, interval) }),
      setMaxReconnectAttempts: (max) => set({ maxReconnectAttempts: Math.max(1, max) }),
      setEntriesPerPage: (count) => set({ entriesPerPage: Math.max(10, count) }),
      setShowOnlyHttps: (show) => set({ showOnlyHttps: show }),
      setShowOnlySse: (show) => set({ showOnlySse: show }),
      setAutoSaveHar: (autoSave) => set({ autoSaveHar: autoSave }),
      setHarSaveInterval: (interval) => set({ harSaveInterval: Math.max(5, interval) }),
      setFilterHost: (host) => set({ filterHost: host }),
      setFilterMethod: (method: 'all' | 'GET' | 'POST' | 'PUT' | 'DELETE' | 'PATCH') => set({ filterMethod: method }),
      setTheme: (theme) => set({ theme }),
      resetSettings: () => set(defaultSettings),
    }),
    { name: 'settings-store' }
  )
);

const store = create<TrafficState>()(
  devtools(
    (set) => ({
      entries: [],
      selectedId: null,
      detail: undefined,
      loading: false,
      error: null,
      connected: false,
      transport: 'unknown',
      setEntries: (entries) =>
        set({
          entries: [...entries].sort((a, b) => {
            const timeA = a.startTime ? new Date(a.startTime).getTime() : 0;
            const timeB = b.startTime ? new Date(b.startTime).getTime() : 0;
            return timeB - timeA;
          }),
        }),
      addOrUpdateEntry: (entry) =>
        set((state) => {
          const existingIndex = state.entries.findIndex((item) => item.id === entry.id);
          if (existingIndex === -1) {
            return { entries: [entry, ...state.entries] };
          }
          const updated = [...state.entries];
          updated[existingIndex] = { ...updated[existingIndex], ...entry };
          return { entries: updated };
        }),
      selectEntry: (id) => set({ selectedId: id }),
      setDetail: (detail) => set({ detail }),
      mergeDetail: (partial) =>
        set((state) => ({ detail: { ...(state.detail ?? {}), ...partial } })),
      clearDetail: () => set({ detail: undefined }),
      setLoading: (loading) => set({ loading }),
      setError: (error) => set({ error }),
      setConnected: (connected) => set({ connected }),
      setTransport: (transport) => set({ transport }),
      clearEntries: () =>
        set({
          entries: [],
          selectedId: null,
          detail: undefined,
        }),
    }),
    { name: 'traffic-store' }
  )
);

// 导出设置store
export const useSettingsStore = settingsStore;

export const useTrafficStore = store;
