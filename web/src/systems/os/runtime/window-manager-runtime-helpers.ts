import type { QueryCacheNotifyEvent } from "@tanstack/react-query";

import type { WindowManagerRuntimeBinding } from "./window-manager-runtime-core";

/** How long a refused-command notice stays before the surface clears it on its own. */
export const WINDOW_MANAGER_DIAGNOSTIC_TTL_MS = 5_000;

/** How long a window-manager command may wait, queued or in flight, before it is dropped. */
export const WINDOW_MANAGER_COMMAND_DEADLINE_MS = 8_000;

/**
 * What is left of a command's deadline at `now`: counted from the input that authored it when the
 * command carries that time, from now otherwise. Zero or less means it has expired.
 */
export function commandTimeLeft(inputAt: number | undefined, now: number): number {
  return inputAt === undefined
    ? WINDOW_MANAGER_COMMAND_DEADLINE_MS
    : WINDOW_MANAGER_COMMAND_DEADLINE_MS - (now - inputAt);
}

/** Whether a queued command still targets the workspace, profile and client it was issued for. */
export function sameRuntimeBinding(
  current: WindowManagerRuntimeBinding,
  enqueued: WindowManagerRuntimeBinding
): boolean {
  return (
    current.workspaceId === enqueued.workspaceId &&
    current.profileId === enqueued.profileId &&
    current.clientId === enqueued.clientId
  );
}

export function randomWindowManagerId(prefix: string): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return `${prefix}-${globalThis.crypto.randomUUID()}`;
  }
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

/** Whether a query-cache notification carries new data rather than observer bookkeeping. */
export function queryCacheEventChangesData(event: QueryCacheNotifyEvent): boolean {
  switch (event.type) {
    case "added":
      return event.query.state.data !== undefined;
    case "removed":
      return true;
    case "updated":
      return (
        event.action.type === "success" ||
        (event.action.type === "setState" && Object.hasOwn(event.action.state, "data"))
      );
    case "observerAdded":
    case "observerRemoved":
    case "observerResultsUpdated":
    case "observerOptionsUpdated":
      return false;
  }
}
