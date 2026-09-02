import { useContext } from "react";

import { useDocumentVisible } from "@/hooks/use-document-visible";

import { WindowLiveDataContext } from "../contexts/window-live-data-context";
import type { OsDesktopRuntimeStore, OsWindow } from "../lib/os-types";
import { useDesktop } from "./use-desktop";

const LIVE_DATA_WINDOW_BUDGET = 2;

function canOwnLiveData(window: OsWindow, activeDesktopId: string | null): boolean {
  return window.desktopId === activeDesktopId && !window.minimized && window.stackActive;
}

function liveDataOwners(state: OsDesktopRuntimeStore): readonly string[] {
  const owners: string[] = [];
  const append = (windowId: string | null) => {
    if (windowId === null || owners.includes(windowId)) return;
    const window = state.windows[windowId];
    if (window === undefined || !canOwnLiveData(window, state.activeDesktopId)) return;
    owners.push(windowId);
  };

  append(state.focusedId);
  for (const windowId of state.client?.focusOrder ?? []) {
    append(windowId);
    if (owners.length === LIVE_DATA_WINDOW_BUDGET) return owners;
  }

  const unvisited = Object.entries(state.windows).sort(
    ([leftId, left], [rightId, right]) => right.layer - left.layer || leftId.localeCompare(rightId)
  );
  for (const [windowId] of unvisited) {
    append(windowId);
    if (owners.length === LIVE_DATA_WINDOW_BUDGET) break;
  }
  return owners;
}

/** Defaults to live outside the retained-window OS shell. */
export function useCurrentWindowLiveDataEnabled(): boolean {
  return useContext(WindowLiveDataContext);
}

/** Bounds retained-window streams while keeping one visible background window live. */
export function useWindowLiveDataEnabled(windowId: string): boolean {
  const documentVisible = useDocumentVisible();
  const windowLive = useDesktop(state => liveDataOwners(state).includes(windowId));
  return documentVisible && windowLive;
}
