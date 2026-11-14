import { useContext, useEffect, useState } from "react";

import { useDocumentVisible } from "@/hooks/use-document-visible";

import { WindowLiveDataContext } from "../contexts/window-live-data-context";
import type { OsDesktopRuntimeStore, OsWindow } from "../lib/os-types";
import { useDesktop } from "./use-desktop";

const LIVE_DATA_WINDOW_BUDGET = 2;

/**
 * How long a window's place in the live-data budget must hold before its app follows it. Gaining
 * or losing the lease re-renders the app, enables or pauses its reads, opens or closes its
 * streams and runs its preload; a tab the operator only passes through (a burst of tab clicks, a
 * focus answer shown on the way to the newest one) does none of that. It exceeds the focus hold
 * (`FOCUS_HOLD_LIMIT_MS`), the longest a rapid-click burst shows one intermediate tab.
 */
export const LIVE_DATA_SETTLE_MS = 500;

function canOwnLiveData(window: OsWindow, activeDesktopId: string | null): boolean {
  return window.desktopId === activeDesktopId && window.shown;
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

  const windowIds = Object.keys(state.windows);
  while (owners.length < LIVE_DATA_WINDOW_BUDGET) {
    let nextId: string | null = null;
    for (const windowId of windowIds) {
      const window = state.windows[windowId];
      if (
        owners[0] === windowId ||
        owners[1] === windowId ||
        !canOwnLiveData(window, state.activeDesktopId)
      )
        continue;
      if (nextId === null || windowId.localeCompare(nextId) < 0) nextId = windowId;
    }
    if (nextId === null) break;
    owners.push(nextId);
  }
  return owners;
}

/** Defaults to live outside the retained-window OS shell. */
export function useCurrentWindowLiveDataEnabled(): boolean {
  return useContext(WindowLiveDataContext);
}

/**
 * Bounds retained-window streams while keeping one visible background window live. A change in
 * the window's lease applies once it has held for `LIVE_DATA_SETTLE_MS`; a hidden document pauses
 * every window at once.
 */
export function useWindowLiveDataEnabled(windowId: string): boolean {
  const documentVisible = useDocumentVisible();
  const windowLive = useDesktop(state => liveDataOwners(state).includes(windowId));
  const [settled, setSettled] = useState(windowLive);
  useEffect(() => {
    if (settled === windowLive) return undefined;
    const timer = setTimeout(() => setSettled(windowLive), LIVE_DATA_SETTLE_MS);
    return () => clearTimeout(timer);
  }, [settled, windowLive]);
  return documentVisible && settled;
}
