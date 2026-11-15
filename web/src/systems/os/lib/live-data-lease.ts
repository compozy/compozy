import { createAtom } from "@xstate/store";

import type { OsDesktopRuntimeStore, OsWindow } from "./os-types";
import type { WindowManagerProjectionAtom } from "./window-manager-projection";

const LIVE_DATA_WINDOW_BUDGET = 2;

/**
 * How long the windows that own live data must hold before their apps follow them. Gaining or
 * losing the lease re-renders the app, enables or pauses its reads, opens or closes its streams
 * and runs its preload; a tab the operator only passes through (a burst of tab clicks, a focus
 * answer shown on the way to the newest one) does none of that. It exceeds the focus hold
 * (`FOCUS_HOLD_LIMIT_MS`), the longest a rapid-click burst shows one intermediate tab.
 */
export const LIVE_DATA_SETTLE_MS = 500;

function canOwnLiveData(window: OsWindow, activeDesktopId: string | null): boolean {
  return window.desktopId === activeDesktopId && window.shown;
}

type LiveDataState = Pick<OsDesktopRuntimeStore, "windows" | "focusedId" | "activeDesktopId"> & {
  client?: Pick<NonNullable<OsDesktopRuntimeStore["client"]>, "focusOrder"> | null;
};

/** The focused window and the most recent visible background window on the active desktop. */
export function liveDataOwners(state: LiveDataState): readonly string[] {
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

function sameOwners(left: readonly string[], right: readonly string[]): boolean {
  if (left.length !== right.length) return false;
  const expected = new Set(right);
  return left.every(windowId => expected.has(windowId));
}

type SettledOwners = ReturnType<typeof createAtom<readonly string[]>>;

const leases = new WeakMap<WindowManagerProjectionAtom, SettledOwners>();

/**
 * The live-data owners of a desktop projection once they have held for `LIVE_DATA_SETTLE_MS`.
 * One lease per projection: a single timer settles every window together, and an app reading it
 * renders only when its own settled lease changes, never on the owners a switch passes through.
 * A pending change restarts its wait only when the owners it waits for change, so frequent
 * publications that keep the same owners still settle.
 */
export function settledLiveDataOwners(projection: WindowManagerProjectionAtom): SettledOwners {
  const existing = leases.get(projection);
  if (existing !== undefined) return existing;
  const settled = createAtom<readonly string[]>(liveDataOwners(projection.get()));
  let pending: readonly string[] | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;
  const stopTimer = () => {
    if (timer !== null) clearTimeout(timer);
    timer = null;
    pending = null;
  };
  projection.subscribe(state => {
    const owners = liveDataOwners(state);
    if (sameOwners(owners, settled.get())) {
      stopTimer();
      return;
    }
    if (pending !== null && sameOwners(owners, pending)) return;
    stopTimer();
    pending = owners;
    timer = setTimeout(() => {
      const next = pending;
      stopTimer();
      if (next !== null) settled.set(next);
    }, LIVE_DATA_SETTLE_MS);
  });
  leases.set(projection, settled);
  return settled;
}
