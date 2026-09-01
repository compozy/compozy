import type { QueryCacheNotifyEvent } from "@tanstack/react-query";

/** How long a refused-command notice stays before the surface clears it on its own. */
export const WINDOW_MANAGER_DIAGNOSTIC_TTL_MS = 5_000;

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
