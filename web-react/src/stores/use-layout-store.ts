import { create } from 'zustand';
import { devtools } from 'zustand/middleware';

type LayoutState = {
  showDetail: boolean;
  showSettings: boolean;
  setShowDetail: (show: boolean) => void;
  setShowSettings: (show: boolean) => void;
  toggleDetail: () => void;
  toggleSettings: () => void;
};

const store = create<LayoutState>()(
  devtools(
    (set) => ({
      showDetail: false,
      showSettings: true,
      setShowDetail: (show) => set({ showDetail: show }),
      setShowSettings: (show) => set({ showSettings: show }),
      toggleDetail: () => set((state) => ({ showDetail: !state.showDetail })),
      toggleSettings: () => set((state) => ({ showSettings: !state.showSettings })),
    }),
    { name: 'layout-store' }
  )
);

export const useLayoutStore = store;
