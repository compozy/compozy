import { create } from "zustand";
import { persist } from "zustand/middleware";

import { normalizeHomeUsageWindow, type HomeUsageWindow } from "../types";

interface HomePrefsStore {
  usageWindow: HomeUsageWindow;
  systemOpen: boolean;
  actions: {
    setUsageWindow: (window: HomeUsageWindow) => void;
    setSystemOpen: (open: boolean) => void;
  };
}

export const useHomePrefsStore = create<HomePrefsStore>()(
  persist(
    set => ({
      usageWindow: 30,
      systemOpen: false,
      actions: {
        setUsageWindow: window => set({ usageWindow: normalizeHomeUsageWindow(window) }),
        setSystemOpen: open => set({ systemOpen: open }),
      },
    }),
    {
      name: "agh:home-prefs",
      partialize: state => ({ usageWindow: state.usageWindow, systemOpen: state.systemOpen }),
      merge: (persisted, current) => {
        const incoming = (persisted ?? {}) as Partial<HomePrefsStore>;
        return {
          ...current,
          ...incoming,
          usageWindow: normalizeHomeUsageWindow(incoming.usageWindow ?? current.usageWindow),
          actions: current.actions,
        };
      },
    }
  )
);

export const useHomeUsageWindow = (): HomeUsageWindow =>
  useHomePrefsStore(state => state.usageWindow);
export const useHomeSystemOpen = (): boolean => useHomePrefsStore(state => state.systemOpen);
export const useHomePrefsActions = () => useHomePrefsStore(state => state.actions);
