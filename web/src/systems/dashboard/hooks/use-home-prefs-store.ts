import { createStore } from "@xstate/store";
import { persist } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import { normalizeHomeUsageWindow, type HomeUsageWindow } from "../types";

interface HomePrefsContext {
  usageWindow: HomeUsageWindow;
  systemOpen: boolean;
}

function mergeHomePrefs(
  persisted: Partial<HomePrefsContext>,
  current: HomePrefsContext
): HomePrefsContext {
  return {
    usageWindow: normalizeHomeUsageWindow(persisted.usageWindow ?? current.usageWindow),
    systemOpen: persisted.systemOpen ?? current.systemOpen,
  };
}

export const homePrefsStore = createStore({
  context: {
    usageWindow: 30 as HomeUsageWindow,
    systemOpen: false,
  },
  on: {
    usageWindowSelected: (context, event: { usageWindow: number }) => ({
      ...context,
      usageWindow: normalizeHomeUsageWindow(event.usageWindow),
    }),
    systemPanelOpened: context => ({ ...context, systemOpen: true }),
    systemPanelClosed: context => ({ ...context, systemOpen: false }),
  },
}).with(
  persist({
    name: "compozy:home-prefs:v2",
    merge: mergeHomePrefs,
  })
);

export const useHomeUsageWindow = (): HomeUsageWindow =>
  useSelector(homePrefsStore, snapshot => snapshot.context.usageWindow);

export const useHomeSystemOpen = (): boolean =>
  useSelector(homePrefsStore, snapshot => snapshot.context.systemOpen);
