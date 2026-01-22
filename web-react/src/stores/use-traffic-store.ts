import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

import { TrafficDetail, TrafficEntry } from '@/types/traffic';

export type SettingsState = {
  // 主题设置
  theme: 'light' | 'dark' | 'auto';

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
const defaultSettings: Omit<SettingsState, keyof { setTheme: any; resetSettings: any }> = {
  theme: 'auto' as const,
};

// 创建设置store
const settingsStore = create<SettingsState>()(
  devtools(
    (set) => ({
      ...defaultSettings,
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
