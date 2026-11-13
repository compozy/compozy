import type { QueryCacheNotifyEvent } from "@tanstack/react-query";

import { effectiveWindowManagerConfig } from "../lib/window-manager-config";
import type {
  WindowManagerClientView,
  WindowManagerConfig,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
import type { DesktopTransitionIntent } from "../stores/window-manager-store";
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

/**
 * Whether a client view belongs to the runtime's binding. Presentation revisions of two bindings
 * do not compare, so a view of another binding is neither held nor adopted.
 */
export function clientViewInBinding(
  binding: WindowManagerRuntimeBinding | null,
  view: Pick<WindowManagerClientView, "workspaceId" | "clientId">
): binding is WindowManagerRuntimeBinding {
  return (
    binding !== null &&
    view.workspaceId === binding.workspaceId &&
    view.clientId === binding.clientId
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

/** Which way a desktop switch slides: toward a later or an earlier desktop in pager order. */
export function desktopTransitionDirection(
  desktops: readonly { id: string; order: number }[],
  fromDesktopId: string,
  toDesktopId: string
): "earlier" | "later" {
  const fromOrder = desktops.find(desktop => desktop.id === fromDesktopId)?.order ?? 0;
  const toOrder = desktops.find(desktop => desktop.id === toDesktopId)?.order ?? 0;
  return toOrder >= fromOrder ? "later" : "earlier";
}

/** The slide a reported desktop switch animates; none without a layout, config, or motion. */
export function desktopTransitionIntent(
  snapshot: WindowManagerSnapshot | null,
  globalConfig: WindowManagerConfig | null,
  reduceMotion: boolean,
  fromDesktopId: string,
  toDesktopId: string
): DesktopTransitionIntent | null {
  if (snapshot === null || globalConfig === null) return null;
  const config = effectiveWindowManagerConfig(globalConfig, snapshot.overrides);
  if (reduceMotion || config.desktopTransition === "instant") return null;
  return {
    fromDesktopId,
    toDesktopId,
    direction: desktopTransitionDirection(snapshot.desktops, fromDesktopId, toDesktopId),
    mode: config.desktopTransition,
  };
}
