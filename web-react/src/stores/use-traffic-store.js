import { create } from 'zustand';
import { devtools } from 'zustand/middleware';
// 默认设置值
const defaultSettings = {
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
    theme: 'auto',
};
// 创建设置store
const settingsStore = create()(devtools((set) => ({
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
    setFilterMethod: (method) => set({ filterMethod: method }),
    setTheme: (theme) => set({ theme }),
    resetSettings: () => set(defaultSettings),
}), { name: 'settings-store' }));
const store = create()(devtools((set) => ({
    entries: [],
    selectedId: null,
    detail: undefined,
    loading: false,
    error: null,
    connected: false,
    transport: 'unknown',
    setEntries: (entries) => set({
        entries: [...entries].sort((a, b) => {
            const timeA = a.startTime ? new Date(a.startTime).getTime() : 0;
            const timeB = b.startTime ? new Date(b.startTime).getTime() : 0;
            return timeB - timeA;
        }),
    }),
    addOrUpdateEntry: (entry) => set((state) => {
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
    mergeDetail: (partial) => set((state) => ({ detail: { ...(state.detail ?? {}), ...partial } })),
    clearDetail: () => set({ detail: undefined }),
    setLoading: (loading) => set({ loading }),
    setError: (error) => set({ error }),
    setConnected: (connected) => set({ connected }),
    setTransport: (transport) => set({ transport }),
    clearEntries: () => set({
        entries: [],
        selectedId: null,
        detail: undefined,
    }),
}), { name: 'traffic-store' }));
// 导出设置store
export const useSettingsStore = settingsStore;
export const useTrafficStore = store;
